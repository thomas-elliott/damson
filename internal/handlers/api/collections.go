package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/audit"
	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

type collectionView struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AssetCount  int       `json:"asset_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) getCollections(c *gin.Context) {
	lib := getLibrary(c)
	type row struct {
		ID          uint
		Name        string
		Description string
		CreatedAt   time.Time
		UpdatedAt   time.Time
		AssetCount  int
	}
	var rows []row
	err := h.db.WithContext(c.Request.Context()).
		Table("collections AS c").
		Select(`c.id, c.name, c.description, c.created_at, c.updated_at,
			COALESCE(COUNT(ca.asset_id), 0) AS asset_count`).
		Joins("LEFT JOIN collection_assets ca ON ca.collection_id = c.id").
		Where("c.library_id = ?", lib.ID).
		Group("c.id").
		Order("c.name").
		Scan(&rows).Error
	if err != nil {
		respond500(c, err)
		return
	}
	out := make([]collectionView, len(rows))
	for i, r := range rows {
		out[i] = collectionView{
			ID: r.ID, Name: r.Name, Description: r.Description,
			AssetCount: r.AssetCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"collections": out})
}

type collectionWriteRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (h *Handler) postCollection(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	var req collectionWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respond400(c, asError("name required"))
		return
	}
	coll := models.Collection{
		LibraryID:       lib.ID,
		Name:            name,
		Description:     req.Description,
		CreatedByUserID: &u.ID,
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var existing models.Collection
		if err := tx.Where("library_id = ? AND LOWER(name) = LOWER(?)", lib.ID, name).First(&existing).Error; err == nil {
			return errConflictf("collection name already exists")
		} else if !isNotFound(err) {
			return err
		}
		if err := tx.Create(&coll).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "collection.create",
			TargetType: "collection",
			TargetID:   &coll.ID,
			Diff:       audit.Created(map[string]any{"name": coll.Name}),
		})
	})
	if err != nil {
		respondGORM(c, err, "collection not found")
		return
	}
	c.JSON(http.StatusCreated, collectionView{
		ID: coll.ID, Name: coll.Name, Description: coll.Description,
		CreatedAt: coll.CreatedAt, UpdatedAt: coll.UpdatedAt,
	})
}

func (h *Handler) getCollection(c *gin.Context) {
	lib := getLibrary(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var coll models.Collection
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND library_id = ?", id, lib.ID).
		First(&coll).Error; err != nil {
		if isNotFound(err) {
			respond404(c, "collection not found")
			return
		}
		respond500(c, err)
		return
	}
	var count int64
	if err := h.db.WithContext(c.Request.Context()).
		Model(&models.CollectionAsset{}).
		Where("collection_id = ?", coll.ID).
		Count(&count).Error; err != nil {
		respond500(c, err)
		return
	}
	c.JSON(http.StatusOK, collectionView{
		ID: coll.ID, Name: coll.Name, Description: coll.Description,
		AssetCount: int(count), CreatedAt: coll.CreatedAt, UpdatedAt: coll.UpdatedAt,
	})
}

func (h *Handler) patchCollection(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req collectionWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	var out models.Collection
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var coll models.Collection
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&coll).Error; err != nil {
			return err
		}
		before := map[string]any{}
		updates := map[string]any{}
		if name := strings.TrimSpace(req.Name); name != "" {
			before["name"] = coll.Name
			coll.Name = name
			updates["name"] = name
		}
		if req.Description != "" {
			before["description"] = coll.Description
			coll.Description = req.Description
			updates["description"] = req.Description
		}
		if len(updates) > 0 {
			if err := tx.Model(&coll).Updates(updates).Error; err != nil {
				return err
			}
		}
		out = coll
		if len(updates) == 0 {
			return nil
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "collection.update",
			TargetType: "collection",
			TargetID:   &coll.ID,
			Diff:       audit.Updated(before, updates),
		})
	})
	if err != nil {
		respondGORM(c, err, "collection not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          out.ID,
		"name":        out.Name,
		"description": out.Description,
	})
}

func (h *Handler) deleteCollection(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var coll models.Collection
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&coll).Error; err != nil {
			return err
		}
		if err := tx.Where("collection_id = ?", coll.ID).Delete(&models.CollectionAsset{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&coll).Error; err != nil {
			return err
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "collection.delete",
			TargetType: "collection",
			TargetID:   &coll.ID,
			Diff:       audit.Deleted(map[string]any{"name": coll.Name}),
		})
	})
	if err != nil {
		respondGORM(c, err, "collection not found")
		return
	}
	c.Status(http.StatusNoContent)
}

type collectionAssetRequest struct {
	AssetID uint `json:"asset_id" binding:"required"`
}

func (h *Handler) postCollectionAsset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req collectionAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond400(c, err)
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var coll models.Collection
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&coll).Error; err != nil {
			return err
		}
		var asset models.Asset
		if err := tx.Where("id = ? AND library_id = ?", req.AssetID, lib.ID).First(&asset).Error; err != nil {
			return err
		}
		link := models.CollectionAsset{CollectionID: coll.ID, AssetID: asset.ID, AddedByUserID: &u.ID}
		res := tx.Where("collection_id = ? AND asset_id = ?", coll.ID, asset.ID).
			Attrs(&link).FirstOrCreate(&link)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // idempotent
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "collection.add",
			TargetType: "collection",
			TargetID:   &coll.ID,
			Diff:       audit.Created(map[string]any{"asset_id": asset.ID}),
		})
	})
	if err != nil {
		if isNotFound(err) {
			respond404(c, "collection or asset not found")
			return
		}
		respond500(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) deleteCollectionAsset(c *gin.Context) {
	lib := getLibrary(c)
	u := auth.GetUser(c)
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	assetID, ok := parseIDParam(c, "assetId")
	if !ok {
		return
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var coll models.Collection
		if err := tx.Where("id = ? AND library_id = ?", id, lib.ID).First(&coll).Error; err != nil {
			return err
		}
		res := tx.Where("collection_id = ? AND asset_id = ?", coll.ID, assetID).
			Delete(&models.CollectionAsset{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // idempotent
		}
		return audit.Write(tx, audit.Entry{
			LibraryID:  &lib.ID,
			UserID:     &u.ID,
			ActionType: "collection.remove",
			TargetType: "collection",
			TargetID:   &coll.ID,
			Diff:       audit.Deleted(map[string]any{"asset_id": assetID}),
		})
	})
	if err != nil {
		respondGORM(c, err, "collection not found")
		return
	}
	c.Status(http.StatusNoContent)
}
