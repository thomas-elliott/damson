import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { OBJLoader } from "three/examples/jsm/loaders/OBJLoader.js";
import { ColladaLoader } from "three/examples/jsm/loaders/ColladaLoader.js";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { FBXLoader } from "three/examples/jsm/loaders/FBXLoader.js";

import { fetchBlob } from "@/api";
import type { FileNode } from "@/design-system";

import { type MeshFormat, meshFormat } from "./previewKind";

async function loadModel(format: MeshFormat, data: ArrayBuffer): Promise<THREE.Object3D> {
  switch (format) {
    case "obj":
      return new OBJLoader().parse(new TextDecoder().decode(data));
    case "dae": {
      const collada = new ColladaLoader().parse(new TextDecoder().decode(data), "");
      if (!collada?.scene) throw new Error("collada: no scene");
      return collada.scene;
    }
    case "gltf":
      return (await new GLTFLoader().parseAsync(data, "")).scene;
    case "fbx":
      return new FBXLoader().parse(data, "");
  }
}

// Position the camera so the whole model is framed, and point the orbit pivot
// at its centre.
function frame(object: THREE.Object3D, camera: THREE.PerspectiveCamera, controls: OrbitControls) {
  const box = new THREE.Box3().setFromObject(object);
  const size = box.getSize(new THREE.Vector3());
  const center = box.getCenter(new THREE.Vector3());
  const maxDim = Math.max(size.x, size.y, size.z) || 1;
  const dist = maxDim * 2.2;
  camera.near = maxDim / 100;
  camera.far = maxDim * 100;
  camera.position.set(center.x + dist, center.y + dist * 0.6, center.z + dist);
  camera.updateProjectionMatrix();
  controls.target.copy(center);
  controls.update();
}

/**
 * Interactive 3D viewer (three.js) for obj/dae/glb/gltf and best-effort FBX.
 * Parses the file from its bytes — no model URL — and renders it with orbit
 * controls. Default export for lazy loading; three.js stays out of the main
 * bundle. `.blend` has no web loader and is download-only (never routed here).
 */
export default function MeshPreview({ node }: { node: FileNode }) {
  const mountRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const mount = mountRef.current;
    const format = meshFormat(node);
    if (!mount || !node.objectKey || !format) return;

    let disposed = false;
    let raf = 0;
    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(window.devicePixelRatio);
    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x1a1814);
    const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 10000);
    scene.add(new THREE.AmbientLight(0xffffff, 1.1));
    const key = new THREE.DirectionalLight(0xffffff, 1.8);
    key.position.set(3, 5, 4);
    scene.add(key);

    const size = () => {
      const w = mount.clientWidth || 1;
      const h = mount.clientHeight || 1;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    };
    mount.appendChild(renderer.domElement);
    renderer.domElement.className = "h-full w-full";
    size();

    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;

    const ro = new ResizeObserver(size);
    ro.observe(mount);

    const animate = () => {
      raf = requestAnimationFrame(animate);
      controls.update();
      renderer.render(scene, camera);
    };
    animate();

    fetchBlob(`/api/files/${node.objectKey}`)
      .then((blob) => blob.arrayBuffer())
      .then((data) => loadModel(format, data))
      .then((object) => {
        if (disposed) return;
        scene.add(object);
        frame(object, camera, controls);
        setLoading(false);
      })
      .catch(() => {
        if (!disposed) setError(true);
      });

    return () => {
      disposed = true;
      cancelAnimationFrame(raf);
      ro.disconnect();
      controls.dispose();
      renderer.dispose();
      renderer.domElement.remove();
      scene.traverse((obj) => {
        const mesh = obj as THREE.Mesh;
        mesh.geometry?.dispose?.();
        const mat = mesh.material;
        if (Array.isArray(mat)) mat.forEach((m) => m.dispose());
        else mat?.dispose?.();
      });
    };
  }, [node]);

  return (
    <div className="relative h-full w-full bg-background">
      <div ref={mountRef} className="h-full w-full" />
      {loading && !error && (
        <div className="absolute inset-0 grid place-items-center text-ui text-subtle-foreground">
          Loading model…
        </div>
      )}
      {error && (
        <div className="absolute inset-0 grid place-items-center p-4 text-center text-ui text-subtle-foreground">
          Couldn’t load this model — download it to view.
        </div>
      )}
    </div>
  );
}
