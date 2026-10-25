import { apiFetch, apiError, parseJSONResponse, jsonHeaders } from '@/api/client'

export async function fetchDownloaderSettings() {
  const res = await apiFetch('/downloader/settings', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function updateDownloaderSettings(payload) {
  const res = await apiFetch('/downloader/settings', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function updateCloudDrive2Settings(payload) {
  const res = await apiFetch('/downloader/clouddrive2', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function fetchCloudDrive2Token() {
  const res = await apiFetch('/downloader/clouddrive2/token', {
    cache: 'no-store',
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function testCloudDrive2(payload) {
  const res = await apiFetch('/downloader/clouddrive2/test', {
    method: 'POST',
    headers: jsonHeaders,
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function fetchDownloadJobs({ limit = 20, offset = 0, signal } = {}) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  const res = await apiFetch(`/downloads?${params}`, {
    cache: 'no-store',
    signal,
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function createDownloadJob({ magnetUrl }) {
  const res = await apiFetch('/downloads', {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({
      magnet_url: magnetUrl,
    }),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function retryDownloadJob(id) {
  const res = await apiFetch(`/downloads/${encodeURIComponent(id)}/retry`, {
    method: 'POST',
  })
  if (!res.ok) throw await apiError(res)
}

export async function cancelDownloadJob(id) {
  const res = await apiFetch(`/downloads/${encodeURIComponent(id)}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) throw await apiError(res)
}

export async function revealDownloadLocation(id) {
  const res = await apiFetch(`/downloads/${id}/reveal`, { method: 'POST' })
  if (!res.ok) {
    throw await apiError(res)
  }
}

export async function deleteDownloadJob(id) {
  const res = await apiFetch(`/downloads/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw await apiError(res)
}

export async function fetchDownloaderProviders() {
  const res = await apiFetch('/downloader/providers', { cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function fetchDownloaderProvider(provider) {
  const res = await apiFetch(`/downloader/providers/${encodeURIComponent(provider)}`, {
    cache: 'no-store',
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function updateDownloaderProvider(provider, payload) {
  const res = await apiFetch(`/downloader/providers/${encodeURIComponent(provider)}`, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function testDownloaderProvider(provider, payload) {
  const res = await apiFetch(`/downloader/providers/${encodeURIComponent(provider)}/test`, {
    method: 'POST',
    headers: jsonHeaders,
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function updateDownloaderActiveProvider(provider) {
  const res = await apiFetch('/downloader/active-provider', {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify({ provider }),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function moveDownloadJob(id, { directoryId } = {}) {
  const res = await apiFetch(`/downloads/${encodeURIComponent(id)}/move`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(directoryId ? { directory_id: directoryId } : {}),
  })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}
