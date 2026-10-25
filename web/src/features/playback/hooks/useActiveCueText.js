import { useEffect, useState } from 'react'

// Reads the active cue text from video.js so the app can render subtitles with
// its own layout instead of the native track display.
export default function useActiveCueText(player, enabled) {
  const [text, setText] = useState('')

  useEffect(() => {
    if (!player || player.isDisposed()) {
      setText('')
      return undefined
    }
    const update = () => {
      if (!enabled) {
        setText('')
        return
      }
      const tracks = player.textTracks()
      const lines = []
      for (let index = 0; index < tracks.length; index += 1) {
        const track = tracks[index]
        if (track.mode !== 'showing' || !track.activeCues) continue
        for (let cueIndex = 0; cueIndex < track.activeCues.length; cueIndex += 1) {
          const cue = track.activeCues[cueIndex]
          if (cue?.text) lines.push(cue.text)
        }
      }
      setText(lines.join('\n').trim())
    }

    update()
    player.on('timeupdate', update)
    player.on('seeked', update)
    player.on('texttrackchange', update)
    const trackList = player.textTracks()
    trackList?.addEventListener?.('change', update)
    trackList?.addEventListener?.('addtrack', update)
    return () => {
      player.off('timeupdate', update)
      player.off('seeked', update)
      player.off('texttrackchange', update)
      trackList?.removeEventListener?.('change', update)
      trackList?.removeEventListener?.('addtrack', update)
    }
  }, [player, enabled])

  return enabled ? text : ''
}
