import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Menu, MenuItem, useMediaQuery } from '@mui/material'
import { SMALL_VIEWPORT_QUERY } from '@/utils/viewport'
import QueuePlayNextRoundedIcon from '@mui/icons-material/QueuePlayNextRounded'
import SkipPreviousRoundedIcon from '@mui/icons-material/SkipPreviousRounded'
import SkipNextRoundedIcon from '@mui/icons-material/SkipNextRounded'
import PlaybackPlaylist from '@/features/playback/components/PlaybackPlaylist'
import MobilePlayerControls from '@/features/playback/components/MobilePlayerControls'
import usePlayerWindow from '@/features/playback/hooks/usePlayerWindow'
import useActiveCueText from '@/features/playback/hooks/useActiveCueText'
import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import ClosedCaptionIcon from '@mui/icons-material/ClosedCaption'
import ClosedCaptionOffIcon from '@mui/icons-material/ClosedCaptionOffOutlined'
import videojs from 'video.js'
import 'video.js/dist/video-js.css'
import {
  createVideoScreenshot,
  fetchPlaybackInfo,
  fetchVideoSubtitles,
  videoSubtitleUrl,
} from '@/features/video/api'
import { getVideoDisplayName } from '@/utils/display'
import {
  PLAYER_HOTKEY_ACTIONS,
  formatPlayerHotkeyKey,
  normalizePlayerHotkeyKey,
  parsePlayerHotkeys,
} from '@/utils/playerHotkeys'
import { zh } from '@/utils/i18n'
import AppModal from '@/shared/ui/AppModal'
import { getErrorMessage } from '@/utils/errors'
import { selectPlaybackSource, startBrowserPlayback } from '@/utils/browserPlayback'
import { startWatchTracking } from '@/features/playback/watchTime'
import { createBrowserResume } from '@/features/playback/browserResume'
import { createPlaybackSession, reportPlaybackSession } from '@/features/playback/api'
import { fetchTools } from '@/features/settings/api'

const VOLUME_STORAGE_KEY = 'javboss.player.volume'
const SUBTITLE_STORAGE_KEY = 'javboss.player.subtitles'

function readSubtitlesEnabled() {
  try {
    return localStorage.getItem(SUBTITLE_STORAGE_KEY) !== '0'
  } catch {
    return true
  }
}

function writeSubtitlesEnabled(enabled) {
  try {
    localStorage.setItem(SUBTITLE_STORAGE_KEY, enabled ? '1' : '0')
  } catch {
    // Storage may be unavailable.
  }
}
const HOTKEY_HINT_DURATION_MS = 5000

function formatSignedAmount(amount) {
  return amount > 0 ? `+${amount}` : String(amount)
}

export default function PlayerModal({
  video,
  playlist = [],
  currentIndex = 0,
  onSelectVideo,
  startTime = null,
  resumePlayback = true,
  onClose,
  hotkeys = null,
  showHotkeyHint = true,
}) {
  const isOpen = Boolean(video)
  const isMobilePlayer = useMediaQuery(SMALL_VIEWPORT_QUERY)
  const playerWindow = usePlayerWindow(isOpen)
  const [player, setPlayer] = useState(null)
  const activePlaybackRef = useRef(null)
  const stopSourceRef = useRef(null)
  const [videoContainer, setVideoContainer] = useState(null)
  const onEndedRef = useRef(null)
  const [playlistVisible, setPlaylistVisible] = useState(
    () =>
      typeof window.matchMedia !== 'function' || !window.matchMedia('(max-width: 767px)').matches
  )
  const [subtitleTracks, setSubtitleTracks] = useState([])
  const [activeSubtitleId, setActiveSubtitleId] = useState('')
  const [subtitlesEnabled, setSubtitlesEnabled] = useState(readSubtitlesEnabled)
  const [subtitleMenuAnchor, setSubtitleMenuAnchor] = useState(null)
  const subtitleTrackElementsRef = useRef([])
  const onCloseRef = useRef(onClose)
  const hotkeyMapRef = useRef(new Map())
  const screenshotInFlightRef = useRef(false)
  const screenshotNoticeTimerRef = useRef(null)
  const [playbackInfo, setPlaybackInfo] = useState(null)
  const [playbackError, setPlaybackError] = useState('')
  const [loadingPlayback, setLoadingPlayback] = useState(false)
  const [screenshotNotice, setScreenshotNotice] = useState('')
  const [hotkeyHintVisible, setHotkeyHintVisible] = useState(false)
  const normalizedHotkeys = useMemo(() => parsePlayerHotkeys(hotkeys), [hotkeys])
  const hotkeyHintLines = useMemo(() => {
    const lines = normalizedHotkeys.map((item) => {
      const key = formatPlayerHotkeyKey(item.key)
      const amount = formatSignedAmount(item.amount)
      if (item.action === PLAYER_HOTKEY_ACTIONS.SEEK) {
        return zh(`${key}：进度 ${amount} 秒`, `${key}: Seek ${amount} seconds`)
      }
      if (item.action === PLAYER_HOTKEY_ACTIONS.VOLUME) {
        return zh(`${key}：音量 ${amount}%`, `${key}: Volume ${amount}%`)
      }
      return zh(`${key}：截图`, `${key}: Screenshot`)
    })
    lines.push(zh('空格：暂停/继续', 'Space: Pause/Resume'))
    lines.push(zh('ESC：退出播放器', 'ESC: Close player'))
    lines.push(
      zh(
        '你可在「设置 → 播放器 → 网页播放器」里关闭此信息显示',
        'You can hide this message under Settings → Player → Web Player.'
      )
    )
    return lines
  }, [normalizedHotkeys])
  const selectedSource = useMemo(() => {
    if (playbackInfo?.video !== video) return null
    return selectPlaybackSource(playbackInfo, document.createElement('video'))
  }, [playbackInfo, video])

  useEffect(() => {
    onEndedRef.current = () => {
      if (currentIndex + 1 < playlist.length) onSelectVideo?.(currentIndex + 1)
    }
  }, [currentIndex, playlist.length, onSelectVideo])

  useEffect(() => {
    setHotkeyHintVisible(false)
    if (!showHotkeyHint || !video?.id || !selectedSource?.src) return undefined

    setHotkeyHintVisible(true)
    const timer = window.setTimeout(() => setHotkeyHintVisible(false), HOTKEY_HINT_DURATION_MS)
    return () => window.clearTimeout(timer)
  }, [selectedSource?.src, showHotkeyHint, video?.id])

  useEffect(() => {
    hotkeyMapRef.current = new Map(normalizedHotkeys.map((item) => [item.key, item]))
  }, [normalizedHotkeys])

  useEffect(() => {
    onCloseRef.current = onClose
  }, [onClose])

  useEffect(() => {
    return () => {
      if (screenshotNoticeTimerRef.current !== null) {
        window.clearTimeout(screenshotNoticeTimerRef.current)
        screenshotNoticeTimerRef.current = null
      }
    }
  }, [video?.id, video?.location_id])

  useEffect(() => {
    if (!video?.id) {
      setPlaybackInfo(null)
      setPlaybackError('')
      setLoadingPlayback(false)
      setScreenshotNotice('')
      return
    }

    let cancelled = false
    setLoadingPlayback(true)
    setPlaybackError('')
    setPlaybackInfo(null)
    setScreenshotNotice('')

    fetchPlaybackInfo(video.id, { locationId: video.location_id })
      .then((info) => {
        if (cancelled) return
        setPlaybackInfo({ ...info, video })
      })
      .catch((err) => {
        if (cancelled) return
        const message = getErrorMessage(err)
        setPlaybackError(message)
        setPlaybackInfo({ video, sources: [] })
      })
      .finally(() => {
        if (cancelled) return
        setLoadingPlayback(false)
      })

    return () => {
      cancelled = true
    }
  }, [video])

  useEffect(() => {
    if (!isOpen || !videoContainer) return

    // Video.js removes its element on dispose; let React own only the container.
    const videoElement = document.createElement('video-js')
    videoElement.classList.add('video-js', 'vjs-big-play-centered', 'h-full', 'w-full')
    videoElement.setAttribute('playsinline', '')
    videoContainer.appendChild(videoElement)
    const player = videojs(videoElement, {
      controls: false,
      autoplay: false,
      preload: 'auto',
      errorDisplay: false,
    })

    setPlayer(player)
    const playerEl = player.el()
    const savedVolume = (() => {
      try {
        const raw = localStorage.getItem(VOLUME_STORAGE_KEY)
        if (raw == null) return null
        const value = Number.parseFloat(raw)
        return Number.isFinite(value) ? value : null
      } catch {
        return null
      }
    })()

    if (savedVolume != null) {
      player.volume(Math.min(1, Math.max(0, savedVolume)))
    }

    const seekBy = (offsetSeconds) => {
      if (!activePlaybackRef.current) return
      const current = player.currentTime() || 0
      const duration = player.duration()
      let next = current + offsetSeconds
      if (Number.isFinite(duration)) {
        next = Math.min(Math.max(0, next), duration)
      } else {
        next = Math.max(0, next)
      }
      player.currentTime(next)
    }

    const adjustVolume = (delta) => {
      const current = player.volume()
      const next = Math.min(1, Math.max(0, current + delta))
      player.volume(next)
    }

    const captureScreenshot = () => {
      const playback = activePlaybackRef.current
      if (!playback || screenshotInFlightRef.current) return
      const { video, locationId } = playback
      const second = Math.max(0, Number(player.currentTime()) || 0)
      const showNotice = (message, duration = 1600) => {
        // Ignore failures from a previous video or a closed player.
        if (player.isDisposed() || activePlaybackRef.current !== playback) return
        if (screenshotNoticeTimerRef.current !== null) {
          window.clearTimeout(screenshotNoticeTimerRef.current)
        }
        setScreenshotNotice(message)
        screenshotNoticeTimerRef.current = window.setTimeout(() => {
          setScreenshotNotice('')
          screenshotNoticeTimerRef.current = null
        }, duration)
      }
      screenshotInFlightRef.current = true
      showNotice(zh('已截图', 'Screenshot taken'))
      createVideoScreenshot(video.id, { second, locationId })
        .catch((err) => {
          console.error(zh('截图失败', 'Failed to capture screenshot'), err)
          showNotice(zh('截图失败', 'Screenshot failed'), 3000)
        })
        .finally(() => {
          screenshotInFlightRef.current = false
        })
    }

    const handleKeyDown = (event) => {
      const target = event.target
      if (
        target instanceof Element &&
        (target.isContentEditable ||
          target.closest('input, textarea, select, [contenteditable="true"]'))
      ) {
        return
      }
      const key = normalizePlayerHotkeyKey(event.key || '')
      const configured = hotkeyMapRef.current.get(key)
      const markHandled = () => {
        event.preventDefault()
        event.stopPropagation()
      }
      if (
        configured &&
        (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK ||
          configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME ||
          configured.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT)
      ) {
        markHandled()
        if (configured.action === PLAYER_HOTKEY_ACTIONS.SEEK) {
          seekBy(configured.amount)
        } else if (configured.action === PLAYER_HOTKEY_ACTIONS.VOLUME) {
          adjustVolume(configured.amount / 100)
        } else if (configured.action === PLAYER_HOTKEY_ACTIONS.SCREENSHOT) {
          captureScreenshot()
        }
        return
      }
      switch (key) {
        case ' ':
        case 'Spacebar': {
          markHandled()
          if (!activePlaybackRef.current) return
          if (player.paused()) {
            player.play()
          } else {
            player.pause()
          }
          break
        }
        case 'Escape':
          markHandled()
          onCloseRef.current?.()
          break
        default:
          return
      }
    }

    const focusPlayer = () => {
      playerEl?.focus({ preventScroll: true })
    }

    if (playerEl && !playerEl.hasAttribute('tabindex')) {
      playerEl.setAttribute('tabindex', '-1')
    }

    window.addEventListener('keydown', handleKeyDown, true)

    const handleVolumeChange = () => {
      try {
        localStorage.setItem(VOLUME_STORAGE_KEY, String(player.volume()))
      } catch {
        return
      }
    }

    player.ready(focusPlayer)
    player.on('fullscreenchange', focusPlayer)
    player.on('volumechange', handleVolumeChange)

    return () => {
      // Stop per-video work before disposing the shared player, including on unmount.
      stopSourceRef.current?.()
      window.removeEventListener('keydown', handleKeyDown, true)
      player.off('fullscreenchange', focusPlayer)
      player.off('volumechange', handleVolumeChange)
      player.dispose()
      setPlayer((current) => (current === player ? null : current))
    }
  }, [isOpen, videoContainer])

  useEffect(() => {
    if (!player || player.isDisposed() || loadingPlayback || !video || !selectedSource?.src) return

    const playback = { video, locationId: playbackInfo.location_id || video.location_id }
    activePlaybackRef.current = playback
    player.controls(!isMobilePlayer)
    player.autoplay(true)
    const stopWatchTracking = startWatchTracking(player, {
      create: () => createPlaybackSession(video.id, playback.locationId),
      report: (session, total) => reportPlaybackSession(video.id, session, total),
    })
    const resume = createBrowserResume({
      videoId: video.id,
      locationId: playback.locationId,
      enabled: resumePlayback,
    })
    const hasExplicitStart = startTime != null && Number.isFinite(Number(startTime))
    const stopPlayback = startBrowserPlayback(
      player,
      selectedSource,
      playbackInfo.sources.find((source) => source.kind === 'hls'),
      hasExplicitStart ? startTime : resume.position,
      (error) => {
        if (activePlaybackRef.current !== playback) return
        const message = error.message || zh('视频播放失败', 'Video playback failed')
        setPlaybackError(message)
      },
      {
        resume: !hasExplicitStart,
        onPosition: resume.record,
        onEnded: resume.complete,
        beforeTranscode: async () => {
          const tools = await fetchTools()
          if (!tools.ffmpeg?.installed && !tools.ffmpeg?.upgrade_available) {
            throw new Error(
              zh(
                '此视频需要转码播放，但尚未安装 FFmpeg。请前往「设置 → 工具」下载 FFmpeg，安装完成后重新打开视频。',
                'This video requires transcoding, but FFmpeg is not installed. Download FFmpeg in Settings → Tools, then reopen the video after installation.'
              )
            )
          }
        },
      }
    )
    window.addEventListener('pagehide', resume.flush)
    const handleEnded = () => onEndedRef.current?.()
    player.on('ended', handleEnded)

    const stop = () => {
      if (activePlaybackRef.current !== playback) return
      activePlaybackRef.current = null
      stopSourceRef.current = null
      stopWatchTracking()
      stopPlayback()
      resume.flush()
      window.removeEventListener('pagehide', resume.flush)
      player.off('ended', handleEnded)
      player.autoplay(false)
      player.pause()
      player.controls(false)
      // Unload the old source and its handler even if the next info request fails.
      // reset() keeps the player/fullscreen container but resets audio and rate settings.
      const volume = player.volume()
      const muted = player.muted()
      const playbackRate = player.playbackRate()
      player.reset()
      player.volume(volume)
      player.muted(muted)
      player.playbackRate(playbackRate)
    }
    stopSourceRef.current = stop
    return stop
  }, [
    player,
    video,
    startTime,
    resumePlayback,
    selectedSource,
    playbackInfo,
    loadingPlayback,
    isMobilePlayer,
  ])

  const playbackLocationId =
    playbackInfo?.video?.id === video?.id ? playbackInfo?.location_id : undefined

  useEffect(() => {
    if (!isOpen || !video?.id) {
      setSubtitleTracks([])
      setActiveSubtitleId('')
      return undefined
    }
    const controller = new AbortController()
    fetchVideoSubtitles(video.id, { locationId: playbackLocationId, signal: controller.signal })
      .then((payload) => {
        const tracks = Array.isArray(payload?.tracks) ? payload.tracks : []
        setSubtitleTracks(tracks)
        setActiveSubtitleId((current) =>
          tracks.some((track) => track.id === current) ? current : tracks[0]?.id || ''
        )
      })
      .catch((error) => {
        if (error?.name === 'AbortError') return
        setSubtitleTracks([])
        setActiveSubtitleId('')
      })
    return () => controller.abort()
  }, [isOpen, video?.id, playbackLocationId])

  useEffect(() => {
    if (!player || player.isDisposed() || !video?.id) return undefined
    const elements = subtitleTracks
      .map((track) => {
        try {
          return player.addRemoteTextTrack(
            {
              kind: 'subtitles',
              src: videoSubtitleUrl(video.id, track.id, playbackLocationId),
              srclang: track.language || 'zh',
              label: track.label,
            },
            false
          )
        } catch {
          return null
        }
      })
      .filter(Boolean)
    subtitleTrackElementsRef.current = elements
    return () => {
      elements.forEach((element) => {
        try {
          player.removeRemoteTextTrack(element)
        } catch {
          // The player may already be disposed.
        }
      })
      subtitleTrackElementsRef.current = []
    }
  }, [player, subtitleTracks, video?.id, playbackLocationId])

  useEffect(() => {
    if (!player || player.isDisposed()) return undefined
    const elements = subtitleTrackElementsRef.current
    const activeIndex = subtitleTracks.findIndex((track) => track.id === activeSubtitleId)
    const apply = () => {
      elements.forEach((element, index) => {
        const track = element?.track
        if (!track) return
        track.mode = subtitlesEnabled && index === activeIndex ? 'showing' : 'disabled'
      })
    }
    apply()
    const timer = window.setTimeout(apply, 150)
    return () => window.clearTimeout(timer)
  }, [player, subtitleTracks, activeSubtitleId, subtitlesEnabled])

  const applySubtitlesEnabled = (enabled) => {
    setSubtitlesEnabled(enabled)
    writeSubtitlesEnabled(enabled)
  }

  const selectSubtitle = (trackId) => {
    setActiveSubtitleId(trackId)
    applySubtitlesEnabled(true)
  }

  const subtitleText = useActiveCueText(player, subtitlesEnabled)

  if (!video) return null

  const displayName = getVideoDisplayName(video)
  const playbackLoading = loadingPlayback || playbackInfo?.video !== video

  if (isMobilePlayer) {
    return (
      <AppModal
        ariaLabel={displayName || zh('视频播放', 'Video playback')}
        backdropColor="rgba(0, 0, 0, 0.95)"
        contentClassName="player-window player-window--mobile"
        onClose={onClose}
        zIndex={1700}
      >
        <div className="mobile-player">
          <div ref={setVideoContainer} data-vjs-player className="mobile-player__video" />
          <MobilePlayerControls
            player={player}
            title={displayName}
            playlist={playlist}
            currentIndex={currentIndex}
            onSelectVideo={onSelectVideo}
            onClose={onClose}
            subtitleTracks={subtitleTracks}
            subtitlesEnabled={subtitlesEnabled}
            activeSubtitleId={activeSubtitleId}
            onSelectSubtitle={selectSubtitle}
            onDisableSubtitles={() => applySubtitlesEnabled(false)}
            subtitleText={subtitleText}
          />
          {playbackLoading ? (
            <div className="mobile-player__loading" aria-live="polite">
              {zh('加载播放信息中…', 'Loading playback info...')}
            </div>
          ) : null}
          {playbackError ? (
            <div className="mobile-player__error" role="alert">
              {playbackError}
            </div>
          ) : null}
        </div>
      </AppModal>
    )
  }

  return (
    <AppModal
      ariaLabel={displayName || zh('视频播放', 'Video playback')}
      backdropColor="rgba(0, 0, 0, 0.7)"
      contentClassName="player-window rounded-lg bg-white shadow-lg"
      contentProps={{ style: playerWindow.style, ...playerWindow.pointerProps }}
      onClose={onClose}
      zIndex={1700}
    >
      <div className="flex h-full min-h-0 flex-col gap-1.5 p-2">
        <header
          className="flex min-w-0 shrink-0 cursor-move touch-none select-none items-center gap-2"
          onPointerDown={playerWindow.onMoveStart}
          title={zh('拖动标题栏移动播放器', 'Drag the title bar to move the player')}
        >
          <h2
            className="min-w-0 flex-1 truncate text-xs font-semibold leading-4"
            title={displayName}
          >
            {displayName}
          </h2>
          {playlist.length > 1 ? (
            <>
              <button
                type="button"
                aria-label={zh('上一个视频', 'Previous video')}
                title={zh('上一个视频', 'Previous video')}
                disabled={currentIndex === 0}
                onClick={() => onSelectVideo?.(currentIndex - 1)}
                className="inline-flex h-6 w-5 shrink-0 items-center justify-center rounded text-zinc-600 hover:bg-zinc-100 disabled:opacity-30"
              >
                <SkipPreviousRoundedIcon fontSize="small" />
              </button>
              <span className="shrink-0 text-xs text-zinc-500">
                {currentIndex + 1} / {playlist.length}
              </span>
              <button
                type="button"
                aria-label={zh('下一个视频', 'Next video')}
                title={zh('下一个视频', 'Next video')}
                disabled={currentIndex === playlist.length - 1}
                onClick={() => onSelectVideo?.(currentIndex + 1)}
                className="inline-flex h-6 w-5 shrink-0 items-center justify-center rounded text-zinc-600 hover:bg-zinc-100 disabled:opacity-30"
              >
                <SkipNextRoundedIcon fontSize="small" />
              </button>
              <button
                type="button"
                aria-label={zh('播放列表', 'Playlist')}
                title={zh('播放列表', 'Playlist')}
                aria-expanded={playlistVisible}
                aria-controls="browser-playlist"
                onClick={() => setPlaylistVisible((visible) => !visible)}
                className="inline-flex h-6 w-5 shrink-0 items-center justify-center rounded text-zinc-600 hover:bg-zinc-100"
              >
                <QueuePlayNextRoundedIcon sx={{ fontSize: 18 }} />
              </button>
            </>
          ) : null}
          {subtitleTracks.length > 0 ? (
            <>
              <button
                type="button"
                aria-label={zh('字幕', 'Subtitles')}
                title={zh('字幕', 'Subtitles')}
                aria-haspopup="menu"
                aria-expanded={Boolean(subtitleMenuAnchor)}
                onClick={(event) => setSubtitleMenuAnchor(event.currentTarget)}
                className={`inline-flex h-6 w-6 shrink-0 items-center justify-center rounded transition hover:bg-zinc-100 ${
                  subtitlesEnabled ? 'text-blue-600' : 'text-zinc-500'
                }`}
              >
                {subtitlesEnabled ? (
                  <ClosedCaptionIcon sx={{ fontSize: 18 }} />
                ) : (
                  <ClosedCaptionOffIcon sx={{ fontSize: 18 }} />
                )}
              </button>
              <Menu
                anchorEl={subtitleMenuAnchor}
                open={Boolean(subtitleMenuAnchor)}
                onClose={() => setSubtitleMenuAnchor(null)}
                disableScrollLock
              >
                <MenuItem
                  selected={!subtitlesEnabled}
                  onClick={() => {
                    applySubtitlesEnabled(false)
                    setSubtitleMenuAnchor(null)
                  }}
                >
                  {zh('关闭字幕', 'Subtitles off')}
                </MenuItem>
                {subtitleTracks.map((track) => (
                  <MenuItem
                    key={track.id}
                    selected={subtitlesEnabled && track.id === activeSubtitleId}
                    onClick={() => {
                      selectSubtitle(track.id)
                      setSubtitleMenuAnchor(null)
                    }}
                  >
                    {track.label}
                  </MenuItem>
                ))}
              </Menu>
            </>
          ) : null}
          <button
            type="button"
            aria-label={zh('关闭', 'Close')}
            title={zh('关闭', 'Close')}
            onClick={onClose}
            className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-zinc-500 transition hover:bg-zinc-100 hover:text-zinc-800 focus:outline-none"
          >
            <CloseRoundedIcon sx={{ fontSize: 16 }} />
          </button>
        </header>
        <div className="flex min-h-0 min-w-0 flex-1">
          <div className="player-shell relative min-w-0 flex-1 bg-black">
            {!playbackError && (screenshotNotice || hotkeyHintVisible) ? (
              <div className="pointer-events-none absolute left-3 top-3 z-10 flex max-w-[calc(100%-1.5rem)] flex-col items-start gap-2">
                {screenshotNotice ? (
                  <div className="rounded bg-black/75 px-3 py-1.5 text-sm font-medium text-white shadow">
                    {screenshotNotice}
                  </div>
                ) : null}
                {hotkeyHintVisible ? (
                  <div className="max-h-[calc(100vh-12rem)] overflow-hidden rounded bg-black/75 px-3 py-2 text-xs leading-5 text-white shadow">
                    {hotkeyHintLines.map((line, index) => (
                      <div key={`${index}-${line}`}>{line}</div>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}
            <div ref={setVideoContainer} data-vjs-player className="h-full w-full" />
            {playbackLoading ? (
              <div
                data-player-loading
                className="absolute inset-0 z-20 flex items-center justify-center bg-black text-sm text-white"
              >
                {zh('加载播放信息中…', 'Loading playback info...')}
              </div>
            ) : null}
            {subtitleText ? <div className="player-subtitle">{subtitleText}</div> : null}
            {playbackError && player && !player.isDisposed()
              ? createPortal(
                  <div className="absolute inset-0 z-20 flex items-center justify-center bg-black/80 p-6">
                    <div
                      role="alert"
                      className="max-w-lg text-center text-sm leading-6 text-red-200"
                    >
                      {playbackError}
                    </div>
                  </div>,
                  player.el()
                )
              : null}
          </div>
          {playlist.length > 1 && playlistVisible ? (
            <PlaybackPlaylist
              items={playlist}
              currentIndex={currentIndex}
              onSelect={onSelectVideo}
            />
          ) : null}
        </div>
      </div>
      {['n', 'e', 's', 'w', 'ne', 'se', 'sw', 'nw'].map((direction) => (
        <div
          key={direction}
          aria-hidden="true"
          data-player-resize={direction}
          className="player-window-resize"
          onPointerDown={(event) => playerWindow.onResizeStart(event, direction)}
        />
      ))}
    </AppModal>
  )
}
