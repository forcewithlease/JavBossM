import { useEffect, useState } from 'react'
import JavPrefixModal from '@/features/jav/components/JavPrefixModal'
import { fetchJavPrefixes } from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'

// Owns the prefix list request so both the desktop rail and the mobile tab bar
// can open the same modal without duplicating state.
export default function JavPrefixModalLauncher({
  open,
  activePrefix = '',
  buildPrefixUrl,
  onSelectPrefix,
  onClose,
}) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    setLoading(true)
    setError('')
    fetchJavPrefixes()
      .then((result) => {
        if (!cancelled) setItems(Array.isArray(result) ? result : [])
      })
      .catch((err) => {
        if (!cancelled) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  return (
    <JavPrefixModal
      open={open}
      items={items}
      loading={loading}
      error={error}
      activePrefix={activePrefix}
      buildPrefixUrl={buildPrefixUrl}
      onSelectPrefix={(item) => {
        onClose?.()
        onSelectPrefix?.(item)
      }}
      onClose={onClose}
    />
  )
}
