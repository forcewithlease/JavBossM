import { useCallback, useEffect, useRef, useState } from 'react'
import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded'
import ClosedCaptionIcon from '@mui/icons-material/ClosedCaption'
import ClosedCaptionOffIcon from '@mui/icons-material/ClosedCaptionOffOutlined'
import FullscreenExitRoundedIcon from '@mui/icons-material/FullscreenExitRounded'
import FullscreenRoundedIcon from '@mui/icons-material/FullscreenRounded'
import PauseRoundedIcon from '@mui/icons-material/PauseRounded'
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded'
import PlaylistPlayRoundedIcon from '@mui/icons-material/PlaylistPlayRounded'
import SkipNextRoundedIcon from '@mui/icons-material/SkipNextRounded'
import SkipPreviousRoundedIcon from '@mui/icons-material/SkipPreviousRounded'
import SpeedRoundedIcon from '@mui/icons-material/SpeedRounded'
import { getVideoDisplayName } from '@/utils/display'
import { zh } from '@/utils/i18n'

const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2]
const HIDE_DELAY_MS = 3200
const DOUBLE_TAP_DELAY_MS = 280
const LONG_PRESS_MS = 500
const SEEK_STEP_SECONDS = 10

function formatTime(value) {
  const total = Math.max(0, Math.floor(Number(value) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  const tail = `${String(minutes).padStart(hours > 0 ? 2 : 1, '0')}:${String(seconds).padStart(2, '0')}`
  return hours > 0 ? `${hours}:${tail}` : tail
}

const clamp = (value, min, max) => Math.min(max, Math.max(min, value))

export default function MobilePlayerControls({
  player,
  title,
  playlist = [],
  currentIndex = 0,
  onSelectVideo,
  onClose,
  subtitleTracks = [],
  subtitlesEnabled = false,
  activeSubtitleId = '',
  onSelectSubtitle,
  onDisableSubtitles,
  subtitleText = '',
}) {
  const rootRef = useRef(null)
  const hideTimerRef = useRef(null)
  const hudTimerRef = useRef(null)
  const longPressTimerRef = useRef(null)
  const tapTimerRef = useRef(null)
  const lastTapRef = useRef(0)
  const gestureRef = useRef(null)

  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [rate, setRate] = useState(1)
  const [controlsVisible, setControlsVisible] = useState(true)
  const [hud, setHud] = useState('')
  const [brightness, setBrightness] = useState(1)
  const [playlistOpen, setPlaylistOpen] = useState(false)
  const [subtitleOpen, setSubtitleOpen] = useState(false)
  const [subtitleBottom, setSubtitleBottom] = useState(78)
  const [speedOpen, setSpeedOpen] = useState(false)
  const [fullscreen, setFullscreen] = useState(
    () => typeof document !== 'undefined' && Boolean(document.fullscreenElement)
  )

  const showHud = useCallback((text) => {
    setHud(text)
    if (hudTimerRef.current !== null) window.clearTimeout(hudTimerRef.current)
    hudTimerRef.current = window.setTimeout(() => {
      hudTimerRef.current = null
      setHud('')
    }, 900)
  }, [])

  const clearHideTimer = useCallback(() => {
    if (hideTimerRef.current !== null) {
      window.clearTimeout(hideTimerRef.current)
      hideTimerRef.current = null
    }
  }, [])

  const revealControls = useCallback(() => {
    setControlsVisible(true)
    clearHideTimer()
    hideTimerRef.current = window.setTimeout(() => {
      hideTimerRef.current = null
      setControlsVisible(false)
    }, HIDE_DELAY_MS)
  }, [clearHideTimer])

  useEffect(() => {
    clearHideTimer()
    if (playlistOpen || speedOpen || subtitleOpen) {
      setControlsVisible(true)
      return clearHideTimer
    }
    if (playing) revealControls()
    else setControlsVisible(true)
    return clearHideTimer
  }, [playing, playlistOpen, speedOpen, subtitleOpen, revealControls, clearHideTimer])

  useEffect(
    () => () => {
      clearHideTimer()
      if (hudTimerRef.current !== null) window.clearTimeout(hudTimerRef.current)
      if (longPressTimerRef.current !== null) window.clearTimeout(longPressTimerRef.current)
      if (tapTimerRef.current !== null) window.clearTimeout(tapTimerRef.current)
    },
    [clearHideTimer]
  )

  useEffect(() => {
    if (!player) return undefined
    // Keep subtitles just below the letterboxed video, like a mobile video app.
    const updateSubtitlePosition = () => {
      const video = player.el()?.querySelector('video')
      const stage = rootRef.current
      if (!video || !stage || !video.videoWidth || !video.videoHeight) return
      const videoRect = video.getBoundingClientRect()
      const stageRect = stage.getBoundingClientRect()
      const aspect = video.videoWidth / video.videoHeight
      let contentHeight = videoRect.width / aspect
      if (contentHeight > videoRect.height) contentHeight = videoRect.height
      const contentTop = videoRect.top + (videoRect.height - contentHeight) / 2
      const contentBottom = contentTop + contentHeight
      setSubtitleBottom(Math.max(52, Math.round(stageRect.bottom - contentBottom + 12)))
    }
    updateSubtitlePosition()
    player.on('loadedmetadata', updateSubtitlePosition)
    window.addEventListener('resize', updateSubtitlePosition)
    document.addEventListener('fullscreenchange', updateSubtitlePosition)
    return () => {
      player.off('loadedmetadata', updateSubtitlePosition)
      window.removeEventListener('resize', updateSubtitlePosition)
      document.removeEventListener('fullscreenchange', updateSubtitlePosition)
    }
  }, [player])

  useEffect(() => {
    if (!player) return undefined
    const syncTime = () => setCurrentTime(player.currentTime() || 0)
    const syncDuration = () => setDuration(player.duration() || 0)
    const handlePlay = () => setPlaying(true)
    const handlePause = () => setPlaying(false)
    const handleRate = () => setRate(player.playbackRate())

    setCurrentTime(player.currentTime() || 0)
    setDuration(player.duration() || 0)
    setRate(player.playbackRate())
    setPlaying(!player.paused())

    player.on('play', handlePlay)
    player.on('pause', handlePause)
    player.on('timeupdate', syncTime)
    player.on('seeked', syncTime)
    player.on('durationchange', syncDuration)
    player.on('loadedmetadata', syncDuration)
    player.on('ratechange', handleRate)
    return () => {
      player.off('play', handlePlay)
      player.off('pause', handlePause)
      player.off('timeupdate', syncTime)
      player.off('seeked', syncTime)
      player.off('durationchange', syncDuration)
      player.off('loadedmetadata', syncDuration)
      player.off('ratechange', handleRate)
    }
  }, [player])

  useEffect(() => {
    const handleFullscreenChange = () => setFullscreen(Boolean(document.fullscreenElement))
    document.addEventListener('fullscreenchange', handleFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange)
  }, [])

  const togglePlay = useCallback(() => {
    if (!player) return
    revealControls()
    if (player.paused()) {
      const promise = player.play()
      if (promise && typeof promise.catch === 'function') promise.catch(() => {})
    } else {
      player.pause()
    }
  }, [player, revealControls])

  const seekBy = useCallback(
    (delta) => {
      if (!player) return
      const total = player.duration()
      const next = clamp(
        (player.currentTime() || 0) + delta,
        0,
        Number.isFinite(total) ? total : undefined
      )
      player.currentTime(next)
      setCurrentTime(next)
    },
    [player]
  )

  const toggleFullscreen = useCallback(async () => {
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen?.()
        try {
          screen.orientation?.unlock?.()
        } catch {
          // Orientation locking is optional.
        }
        return
      }
      await rootRef.current?.requestFullscreen?.()
      try {
        await screen.orientation?.lock?.('landscape')
      } catch {
        // Orientation locking is optional.
      }
    } catch {
      // Fullscreen may be blocked by the platform; the modal is already full-screen.
    }
  }, [])

  const changeRate = (next) => {
    if (!player) return
    player.playbackRate(next)
    setRate(next)
    setSpeedOpen(false)
    showHud(zh(`${next}x 倍速`, `${next}x speed`))
    revealControls()
  }

  const handlePointerDown = (event) => {
    if (!player || event.button !== 0) return
    const rect = event.currentTarget.getBoundingClientRect()
    const offsetX = event.clientX - rect.left
    const side =
      offsetX < rect.width * 0.35 ? 'left' : offsetX > rect.width * 0.65 ? 'right' : 'center'
    gestureRef.current = {
      side,
      startX: event.clientX,
      startY: event.clientY,
      startedAt: performance.now(),
      startVolume: player.volume(),
      startBrightness: brightness,
      mode: '',
      longPressed: false,
      pointerId: event.pointerId,
      height: rect.height || 1,
    }
    try {
      event.currentTarget.setPointerCapture?.(event.pointerId)
    } catch {
      // Synthetic pointers and some WebViews do not support capture.
    }
    if (longPressTimerRef.current !== null) window.clearTimeout(longPressTimerRef.current)
    longPressTimerRef.current = window.setTimeout(() => {
      longPressTimerRef.current = null
      const gesture = gestureRef.current
      if (!gesture || gesture.mode) return
      gesture.longPressed = true
      gesture.savedRate = player.playbackRate()
      player.playbackRate(2)
      showHud(zh('2x 倍速播放中', 'Playing at 2x'))
    }, LONG_PRESS_MS)
  }

  const handlePointerMove = (event) => {
    const gesture = gestureRef.current
    if (!player || !gesture) return
    const deltaX = event.clientX - gesture.startX
    const deltaY = event.clientY - gesture.startY
    if (!gesture.mode) {
      if (Math.hypot(deltaX, deltaY) < 14) return
      if (longPressTimerRef.current !== null) {
        window.clearTimeout(longPressTimerRef.current)
        longPressTimerRef.current = null
      }
      gesture.mode =
        Math.abs(deltaY) > Math.abs(deltaX)
          ? gesture.side === 'left'
            ? 'brightness'
            : 'volume'
          : 'ignore'
      if (gesture.side === 'center') gesture.mode = 'ignore'
    }
    const ratio = clamp(-deltaY / gesture.height, -1, 1)
    if (gesture.mode === 'volume') {
      const next = clamp(gesture.startVolume + ratio, 0, 1)
      player.volume(next)
      showHud(`${Math.round(next * 100)}%`)
    } else if (gesture.mode === 'brightness') {
      const next = clamp(gesture.startBrightness + ratio, 0.2, 1)
      setBrightness(next)
      showHud(`${Math.round(next * 100)}%`)
    }
  }

  const handlePointerUp = (event) => {
    const gesture = gestureRef.current
    if (!gesture) return
    if (longPressTimerRef.current !== null) {
      window.clearTimeout(longPressTimerRef.current)
      longPressTimerRef.current = null
    }
    try {
      if (event.currentTarget.hasPointerCapture?.(gesture.pointerId)) {
        event.currentTarget.releasePointerCapture(gesture.pointerId)
      }
    } catch {
      // Capture may already be released by the browser.
    }
    gestureRef.current = null
    if (gesture.longPressed) {
      player?.playbackRate(gesture.savedRate ?? 1)
      setHud('')
      return
    }
    if (gesture.mode) return

    const now = performance.now()
    const recentTap = now - lastTapRef.current < DOUBLE_TAP_DELAY_MS
    const moved = Math.hypot(event.clientX - gesture.startX, event.clientY - gesture.startY) > 12
    lastTapRef.current = now
    if (!recentTap || moved || gesture.side === 'center') {
      if (tapTimerRef.current !== null) window.clearTimeout(tapTimerRef.current)
      tapTimerRef.current = window.setTimeout(() => {
        tapTimerRef.current = null
        setSpeedOpen(false)
        setSubtitleOpen(false)
        setControlsVisible((visible) => !visible)
      }, DOUBLE_TAP_DELAY_MS)
      return
    }
    if (tapTimerRef.current !== null) {
      window.clearTimeout(tapTimerRef.current)
      tapTimerRef.current = null
    }
    lastTapRef.current = 0
    seekBy(gesture.side === 'left' ? -SEEK_STEP_SECONDS : SEEK_STEP_SECONDS)
    showHud(gesture.side === 'left' ? `-${SEEK_STEP_SECONDS}s` : `+${SEEK_STEP_SECONDS}s`)
    revealControls()
  }

  const progress = duration > 0 ? clamp(currentTime / duration, 0, 1) : 0
  const episodes = Array.isArray(playlist) ? playlist : []

  const handleSeekPointer = (event) => {
    if (!player || !duration) return
    const rect = event.currentTarget.getBoundingClientRect()
    const ratio = clamp((event.clientX - rect.left) / rect.width, 0, 1)
    const next = ratio * duration
    player.currentTime(next)
    setCurrentTime(next)
  }

  return (
    <div ref={rootRef} className="mobile-player__stage">
      <div
        className="mobile-player__dim"
        style={{ opacity: clamp(1 - brightness, 0, 0.8) }}
        aria-hidden="true"
      />
      <div
        className="mobile-player__gesture"
        role="presentation"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
      />

      <div
        className={`mobile-player__chrome ${controlsVisible ? '' : 'mobile-player__chrome--hidden'}`}
      >
        <div className="mobile-player__top">
          <button
            type="button"
            className="mobile-player__icon-button"
            onClick={onClose}
            aria-label={zh('返回', 'Back')}
          >
            <ArrowBackRoundedIcon />
          </button>
          <div className="mobile-player__title-block">
            <span className="mobile-player__title">{title}</span>
            {episodes.length > 1 ? (
              <span className="mobile-player__counter">
                {currentIndex + 1} / {episodes.length}
              </span>
            ) : null}
          </div>
          {subtitleTracks.length > 0 ? (
            <button
              type="button"
              className={`mobile-player__cc ${subtitlesEnabled ? 'mobile-player__cc--active' : ''}`}
              onClick={() => {
                setSubtitleOpen(true)
                setSpeedOpen(false)
                setPlaylistOpen(false)
                revealControls()
              }}
              aria-label={zh('字幕', 'Subtitles')}
              aria-haspopup="dialog"
            >
              {subtitlesEnabled ? (
                <ClosedCaptionIcon fontSize="small" />
              ) : (
                <ClosedCaptionOffIcon fontSize="small" />
              )}
            </button>
          ) : null}
        </div>

        <button
          type="button"
          className={`mobile-player__center ${playing ? 'mobile-player__center--playing' : ''}`}
          onClick={togglePlay}
          aria-label={playing ? zh('暂停', 'Pause') : zh('播放', 'Play')}
        >
          {playing ? <PauseRoundedIcon /> : <PlayArrowRoundedIcon />}
        </button>

        {speedOpen ? (
          <div
            className="mobile-player__speed-panel"
            role="dialog"
            aria-label={zh('播放速度', 'Playback speed')}
          >
            {SPEEDS.map((speed) => (
              <button
                key={speed}
                type="button"
                className={Math.abs(speed - rate) < 0.01 ? 'is-active' : ''}
                onClick={() => changeRate(speed)}
              >
                {speed}x
              </button>
            ))}
          </div>
        ) : null}

        <div className="mobile-player__bottom">
          <div
            className="mobile-player__progress"
            role="presentation"
            onPointerDown={(event) => {
              try {
                event.currentTarget.setPointerCapture?.(event.pointerId)
              } catch {
                // Ignore capture failures on synthetic pointers.
              }
              handleSeekPointer(event)
            }}
            onPointerMove={(event) => {
              if (event.currentTarget.hasPointerCapture?.(event.pointerId)) handleSeekPointer(event)
            }}
          >
            <span className="mobile-player__progress-rail" />
            <span
              className="mobile-player__progress-fill"
              style={{ width: `${progress * 100}%` }}
            />
            <span
              className="mobile-player__progress-thumb"
              style={{ left: `${progress * 100}%` }}
            />
          </div>
          <div className="mobile-player__bottom-row">
            <button
              type="button"
              className="mobile-player__icon-button"
              onClick={togglePlay}
              aria-label={playing ? zh('暂停', 'Pause') : zh('播放', 'Play')}
            >
              {playing ? <PauseRoundedIcon /> : <PlayArrowRoundedIcon />}
            </button>
            <span className="mobile-player__time">
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
            {episodes.length > 1 ? (
              <>
                <button
                  type="button"
                  className="mobile-player__icon-button"
                  onClick={() => onSelectVideo?.(currentIndex - 1)}
                  disabled={currentIndex <= 0}
                  aria-label={zh('上一个视频', 'Previous video')}
                >
                  <SkipPreviousRoundedIcon />
                </button>
                <button
                  type="button"
                  className="mobile-player__icon-button"
                  onClick={() => onSelectVideo?.(currentIndex + 1)}
                  disabled={currentIndex >= episodes.length - 1}
                  aria-label={zh('下一个视频', 'Next video')}
                >
                  <SkipNextRoundedIcon />
                </button>
              </>
            ) : null}
            <span className="mobile-player__spacer" />
            <button
              type="button"
              className="mobile-player__text-button"
              onClick={() => {
                setSpeedOpen((open) => !open)
                revealControls()
              }}
              aria-label={zh('播放速度', 'Playback speed')}
              aria-expanded={speedOpen}
            >
              <SpeedRoundedIcon fontSize="small" />
              {rate}x
            </button>
            {episodes.length > 1 ? (
              <button
                type="button"
                className="mobile-player__icon-button"
                onClick={() => {
                  setPlaylistOpen(true)
                  setSpeedOpen(false)
                  setSubtitleOpen(false)
                }}
                aria-label={zh('选集', 'Episodes')}
              >
                <PlaylistPlayRoundedIcon />
              </button>
            ) : null}
            {typeof document !== 'undefined' && document.fullscreenEnabled ? (
              <button
                type="button"
                className="mobile-player__icon-button"
                onClick={toggleFullscreen}
                aria-label={
                  fullscreen ? zh('退出全屏', 'Exit fullscreen') : zh('全屏', 'Fullscreen')
                }
              >
                {fullscreen ? <FullscreenExitRoundedIcon /> : <FullscreenRoundedIcon />}
              </button>
            ) : null}
          </div>
        </div>
      </div>

      {subtitleText ? (
        <div
          className="mobile-player__subtitle"
          style={{ bottom: `${subtitleBottom}px` }}
          aria-live="off"
        >
          {subtitleText}
        </div>
      ) : null}

      {hud ? (
        <div className="mobile-player__hud" aria-live="polite">
          {hud}
        </div>
      ) : null}

      {subtitleOpen ? (
        <div className="mobile-player__sheet-backdrop">
          <div className="mobile-player__sheet" role="dialog" aria-label={zh('字幕', 'Subtitles')}>
            <div className="mobile-player__sheet-header">
              <span>{zh('字幕', 'Subtitles')}</span>
              <button type="button" onClick={() => setSubtitleOpen(false)}>
                {zh('关闭', 'Close')}
              </button>
            </div>
            <ol className="mobile-player__episodes">
              <li>
                <button
                  type="button"
                  className={!subtitlesEnabled ? 'is-active' : ''}
                  onClick={() => {
                    onDisableSubtitles?.()
                    setSubtitleOpen(false)
                  }}
                >
                  <span className="mobile-player__episode-index">
                    <ClosedCaptionOffIcon fontSize="inherit" />
                  </span>
                  <span className="mobile-player__episode-title">
                    {zh('关闭字幕', 'Subtitles off')}
                  </span>
                </button>
              </li>
              {subtitleTracks.map((track) => (
                <li key={track.id}>
                  <button
                    type="button"
                    className={subtitlesEnabled && track.id === activeSubtitleId ? 'is-active' : ''}
                    onClick={() => {
                      onSelectSubtitle?.(track.id)
                      setSubtitleOpen(false)
                    }}
                  >
                    <span className="mobile-player__episode-index">
                      <ClosedCaptionIcon fontSize="inherit" />
                    </span>
                    <span className="mobile-player__episode-title">{track.label}</span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        </div>
      ) : null}

      {playlistOpen ? (
        <div className="mobile-player__sheet-backdrop">
          <div className="mobile-player__sheet" role="dialog" aria-label={zh('选集', 'Episodes')}>
            <div className="mobile-player__sheet-header">
              <span>{zh('选集', 'Episodes')}</span>
              <button type="button" onClick={() => setPlaylistOpen(false)}>
                {zh('关闭', 'Close')}
              </button>
            </div>
            <ol className="mobile-player__episodes">
              {episodes.map((item, index) => (
                <li key={`${item.id}-${item.location_id || 0}-${index}`}>
                  <button
                    type="button"
                    className={index === currentIndex ? 'is-active' : ''}
                    onClick={() => {
                      onSelectVideo?.(index)
                      setPlaylistOpen(false)
                    }}
                  >
                    <span className="mobile-player__episode-index">
                      {index === currentIndex ? (
                        <PlayArrowRoundedIcon fontSize="inherit" />
                      ) : (
                        index + 1
                      )}
                    </span>
                    <span className="mobile-player__episode-title">
                      {getVideoDisplayName(item)}
                    </span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        </div>
      ) : null}
    </div>
  )
}
