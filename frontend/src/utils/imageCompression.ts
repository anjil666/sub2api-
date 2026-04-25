const MAX_DIMENSION = 2048
const JPEG_QUALITY = 0.92

export async function compressImageIfNeeded(file: File): Promise<File> {
  if (!file.type.startsWith('image/')) return file

  const bitmap = await createImageBitmap(file)
  const { width, height } = bitmap

  if (width <= MAX_DIMENSION && height <= MAX_DIMENSION) {
    bitmap.close()
    return file
  }

  const scale = MAX_DIMENSION / Math.max(width, height)
  const newW = Math.round(width * scale)
  const newH = Math.round(height * scale)

  const canvas = new OffscreenCanvas(newW, newH)
  const ctx = canvas.getContext('2d')!
  ctx.drawImage(bitmap, 0, 0, newW, newH)
  bitmap.close()

  const blob = await canvas.convertToBlob({ type: 'image/jpeg', quality: JPEG_QUALITY })
  const ext = file.name.replace(/\.[^.]+$/, '')
  return new File([blob], `${ext}.jpg`, { type: 'image/jpeg' })
}

export function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}
