import ArrowBackRoundedIcon from '@mui/icons-material/ArrowBackRounded'
import ArrowForwardRoundedIcon from '@mui/icons-material/ArrowForwardRounded'
import DisplaySettingsOutlinedIcon from '@mui/icons-material/DisplaySettingsOutlined'
import DownloadOutlinedIcon from '@mui/icons-material/DownloadOutlined'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import NumbersRoundedIcon from '@mui/icons-material/NumbersRounded'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import { PRIMARY_TABS } from '@/app/layout/navigationTabs'
import { zh } from '@/utils/i18n'

function RailButton({
  active = false,
  badge = '',
  badgeTone = '',
  className = '',
  disabled = false,
  icon: Icon,
  label,
  onClick,
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      aria-label={badge ? `${label} (${badge})` : label}
      aria-current={active ? 'page' : undefined}
      className={`side-tabs__button ${active ? 'side-tabs__button--active' : ''} ${className}`}
    >
      <Icon className="side-tabs__icon" fontSize="small" />
      <span className="side-tabs__label">{label}</span>
      {badge ? (
        <span className={`side-tabs__badge side-tabs__badge--${badgeTone}`}>{badge}</span>
      ) : null}
    </button>
  )
}

export default function SideTabs({
  activeTab,
  canGoBack,
  canGoForward,
  isJavMode,
  onBrowserBack,
  onBrowserForward,
  onOpenDownload,
  onOpenGlobalSettings,
  onOpenDisplaySettings,
  onOpenJavPrefix,
  onOpenJavTagModal,
  onOpenTagModal,
  onSelectTab,
  showDirectorySetupHint = false,
}) {
  return (
    <aside className="side-tabs" aria-label={zh('主导航', 'Primary navigation')}>
      <div className="side-tabs__history" aria-label={zh('浏览历史', 'Navigation history')}>
        <button
          type="button"
          onClick={onBrowserBack}
          disabled={!canGoBack}
          aria-label={zh('后退', 'Back')}
        >
          <ArrowBackRoundedIcon fontSize="small" />
        </button>
        <button
          type="button"
          onClick={onBrowserForward}
          disabled={!canGoForward}
          aria-label={zh('前进', 'Forward')}
        >
          <ArrowForwardRoundedIcon fontSize="small" />
        </button>
      </div>

      <nav className="side-tabs__nav">
        {PRIMARY_TABS.map((tab) => (
          <RailButton
            key={tab.id}
            active={activeTab === tab.id}
            icon={tab.icon}
            label={tab.label}
            onClick={() => onSelectTab?.(tab.id)}
          />
        ))}
        <div className="side-tabs__nav-tools">
          <RailButton
            badge={isJavMode ? 'JAV' : zh('视频', 'Video')}
            badgeTone={isJavMode ? 'jav' : 'video'}
            icon={LocalOfferOutlinedIcon}
            label={zh('标签', 'Tags')}
            onClick={isJavMode ? onOpenJavTagModal : onOpenTagModal}
          />
          <RailButton
            icon={NumbersRoundedIcon}
            label={zh('番号', 'JAV codes')}
            onClick={onOpenJavPrefix}
          />
          <RailButton
            icon={DisplaySettingsOutlinedIcon}
            label={zh('显示', 'Display')}
            onClick={onOpenDisplaySettings}
          />
        </div>
      </nav>

      <div className="side-tabs__tools">
        <RailButton
          className="w-full"
          icon={DownloadOutlinedIcon}
          label={zh('下载', 'Downloads')}
          onClick={onOpenDownload}
        />
        <div className="relative w-full">
          {showDirectorySetupHint ? (
            <div className="directory-setup-hint side-tabs__directory-setup-hint" role="status">
              <ArrowBackRoundedIcon
                className="directory-setup-hint__arrow shrink-0"
                fontSize="small"
                aria-hidden="true"
              />
              <span>
                {zh(
                  '您还没有添加目录，点击 “设置” 在 “目录管理” 内添加。',
                  'No directories yet. Click here to add one in Directory Management'
                )}
              </span>
            </div>
          ) : null}
          <RailButton
            className="w-full"
            icon={SettingsOutlinedIcon}
            label={zh('设置', 'Settings')}
            onClick={onOpenGlobalSettings}
          />
        </div>
      </div>
    </aside>
  )
}
