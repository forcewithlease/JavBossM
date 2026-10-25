export const SMALL_VIEWPORT_QUERY =
  '(max-width: 767px), (orientation: landscape) and (max-height: 500px)'

export function isSmallViewport() {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return window.matchMedia(SMALL_VIEWPORT_QUERY).matches
}
