import { isSmallViewport } from '@/utils/viewport'

export function createNavigationSlice({ set }) {
  // Phones start in waterfall mode: it avoids the cramped pagination row.
  const waterfall = isSmallViewport()
  return {
    waterfallModes: {
      video: waterfall,
      jav: waterfall,
      idol: waterfall,
      studio: waterfall,
      series: waterfall,
    },
    setWaterfallModes: (update) =>
      set((state) => ({ waterfallModes: update(state.waterfallModes) })),
    viewMode: 'video',
    javTab: 'list',
    clearLegacyDownloadTab: () => set({ javTab: 'list' }),
    setViewMode: (mode) => {
      if (mode !== 'video' && mode !== 'jav') return
      set({
        viewMode: mode,
        ...(mode === 'jav' ? { videoTempSort: '' } : { javTempSort: '', idolTempSort: '' }),
      })
    },
    setJavTab: (tab) => {
      if (
        tab !== 'list' &&
        tab !== 'idol' &&
        tab !== 'studio' &&
        tab !== 'series' &&
        tab !== 'download'
      )
        return
      set({ javTab: tab, javTempSort: '', idolTempSort: '' })
    },
  }
}
