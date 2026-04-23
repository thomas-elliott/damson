// Import queue view.
function QueueItem({ item, onReview }) {
  const licData = LICENCES[item.detectedLicence] || LICENCES.Unknown;
  const isUploading = item.progress < 100;
  return (
    <div className="queue-item">
      <div className="queue-thumb">
        <TypeIcon type={item.type} size={22} stroke={1.4}/>
      </div>
      <div className="queue-meta">
        <div className="queue-name">{item.name}</div>
        <div className="queue-sub">
          <span>{formatSize(item.size)}</span>
          <span className="sep">·</span>
          <span>{item.detectedType}</span>
          <span className="sep">·</span>
          <span>{item.imported}</span>
        </div>
        {isUploading
          ? <div className="queue-progress"><div style={{ width: item.progress + '%' }}/></div>
          : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 6 }}>
              {item.detectedLicence ? <LicencePill licence={item.detectedLicence} compact/> : null}
              {item.needsFields.length > 0
                ? <span style={{ fontSize: 11, color: 'var(--text-4)' }}>
                    Needs: <span style={{ color: 'var(--warn)' }}>{item.needsFields.join(', ')}</span>
                  </span>
                : <span style={{ fontSize: 11, color: 'var(--ok)' }}>Ready to confirm</span>}
            </div>
          )
        }
      </div>
      {isUploading
        ? <span className="mono" style={{ fontSize: 11, color: 'var(--text-3)' }}>{item.progress}%</span>
        : (
          <>
            <button className="tbtn" style={{ border: '1px solid var(--line)' }} onClick={() => onReview(item)}>
              Review
            </button>
            <button className="tbtn primary">Confirm</button>
          </>
        )}
    </div>
  );
}

function QueueView({ queue, onReview }) {
  return (
    <div className="grid-wrap">
      <div className="page-head">
        <h1>Import queue</h1>
        <p>{queue.length} items awaiting review. Items can be used elsewhere in the library while unreviewed.</p>
      </div>
      <div className="queue-list">
        {queue.length === 0
          ? <div className="queue-empty">Nothing waiting. Drop files anywhere in the library to import.</div>
          : queue.map(q => <QueueItem key={q.id} item={q} onReview={onReview}/>)}
      </div>
    </div>
  );
}

Object.assign(window, { QueueView });
