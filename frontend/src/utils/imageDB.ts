const DB_NAME = 'sub2api-image-studio'
const DB_VERSION = 1
const STORE_NAME = 'images'

export interface ImageRecord {
  id: string
  prompt: string
  model: string
  size: string
  mode: 'generation' | 'single-edit' | 'multi-edit' | 'batch' | 'storyboard'
  imageUrl: string
  thumbnailBlob?: Blob
  groupName: string
  style: string
  createdAt: number
  batchId?: string
  storyboardId?: string
  sceneIndex?: number
}

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' })
        store.createIndex('by-date', 'createdAt')
        store.createIndex('by-mode', 'mode')
        store.createIndex('by-batch', 'batchId')
        store.createIndex('by-storyboard', 'storyboardId')
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function saveImage(record: ImageRecord): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).put(record)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function getImages(
  mode?: ImageRecord['mode'],
  limit = 50,
  offset = 0
): Promise<ImageRecord[]> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const index = store.index('by-date')
    const results: ImageRecord[] = []
    let skipped = 0
    const req = index.openCursor(null, 'prev')
    req.onsuccess = () => {
      const cursor = req.result
      if (!cursor || results.length >= limit) {
        resolve(results)
        return
      }
      const record = cursor.value as ImageRecord
      if (mode && record.mode !== mode) {
        cursor.continue()
        return
      }
      if (skipped < offset) {
        skipped++
        cursor.continue()
        return
      }
      results.push(record)
      cursor.continue()
    }
    req.onerror = () => reject(req.error)
  })
}

export async function deleteImages(ids: string[]): Promise<void> {
  if (!ids.length) return
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    for (const id of ids) store.delete(id)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function clearAllImages(): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).clear()
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function countImages(): Promise<number> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const req = tx.objectStore(STORE_NAME).count()
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}
