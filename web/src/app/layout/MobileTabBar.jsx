import { useState } from 'react'
import Drawer from '@mui/material/Drawer'
import DisplaySettingsOutlinedIcon from '@mui/icons-material/DisplaySettingsOutlined'
import DownloadOutlinedIcon from '@mui/icons-material/DownloadOutlined'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import MoreHorizRoundedIcon from '@mui/icons-material/MoreHorizRounded'
import NumbersRoundedIcon from '@mui/icons-material/NumbersRounded'
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined'
import { PRIMARY_TABS } from '@/app/layout/navigationTabs'
import { zh } from '@/utils/i18n'

// Small-screen counterpart of SideTabs: primary sections stay in thumb reach,
// secondary tools move into a bottom sheet.
export default function MobileTabBar({
  activeTab,
  isJavMode,
  onOpenDownload,
  onOpenGlobalSettings,
  onOpenDisplaySettings,
  onOpenJavPrefix,
  onOpenJavTagModal,
  onOpenTagModal,
  onSelectTab,
  showDirectorySetupHint = false,
}) {
  const [moreOpen, setMoreOpen] = useState(false)

  const closeMore = () => setMoreOpen(false)
  const runAndClose = (action) => () => {
    closeMore()
    action?.()
  }

  const toolItems = [
    {
      id: 'tags',
      label: zh('标签', 'Tags'),
      icon: LocalOfferOutlinedIcon,
      onClick: isJavMode ? onOpenJavTagModal : onOpenTagModal,
    },
    {
      id: 'prefix',
      label: zh('番号', 'JAV codes'),
      icon: NumbersRoundedIcon,
      onClick: onOpenJavPrefix,
    },
    {
      id: 'display',
      label: zh('显示', 'Display'),
      icon: DisplaySettingsOutlinedIcon,
      onClick: onOpenDisplaySettings,
    },
    {
      id: 'downloads',
      label: zh('下载', 'Downloads'),
      icon: DownloadOutlinedIcon,
      onClick: onOpenDownload,
    },
    {
      id: 'settings',
      label: zh('设置', 'Settings'),
      icon: SettingsOutlinedIcon,
      onClick: onOpenGlobalSettings,
    },
  ]

  return (
    <>
      <nav className="mobile-tab-bar" aria-label={zh('主导航', 'Primary navigation')}>
        {PRIMARY_TABS.map((tab) => {
          const Icon = tab.icon
          const active = activeTab === tab.id
          return (
            <button
              key={tab.id}
              type="button"
              className={`mobile-tab-bar__item ${active ? 'mobile-tab-bar__item--active' : ''}`}
              onClick={() => onSelectTab?.(tab.id)}
              aria-current={active ? 'page' : undefined}
            >
              <Icon className="mobile-tab-bar__icon" fontSize="inherit" />
              <span className="mobile-tab-bar__label">{tab.label}</span>
            </button>
          )
        })}
        <button
          type="button"
          className={`mobile-tab-bar__item ${moreOpen ? 'mobile-tab-bar__item--active' : ''} ${
            showDirectorySetupHint ? 'mobile-tab-bar__item--alert' : ''
          }`}
          onClick={() => setMoreOpen(true)}
          aria-haspopup="dialog"
          aria-expanded={moreOpen}
        >
          <MoreHorizRoundedIcon className="mobile-tab-bar__icon" fontSize="inherit" />
          <span className="mobile-tab-bar__label">{zh('更多', 'More')}</span>
        </button>
      </nav>

      <Drawer
        anchor="bottom"
        open={moreOpen}
        onClose={closeMore}
        slotProps={{ paper: { className: 'mobile-more-sheet' } }}
      >
        <div className="mobile-more-sheet__grabber" aria-hidden="true" />
        <div className="mobile-more-sheet__header">
          <h2 className="mobile-more-sheet__title">{zh('更多', 'More')}</h2>
          <button
            type="button"
            className="mobile-more-sheet__close"
            onClick={closeMore}
            aria-label={zh('关闭', 'Close')}
          >
            {zh('关闭', 'Close')}
          </button>
        </div>

        <div className="mobile-more-sheet__tools">
          {toolItems.map(({ id, label, icon: Icon, onClick }) => (
            <button
              key={id}
              type="button"
              className="mobile-more-sheet__tool"
              onClick={runAndClose(onClick)}
            >
              <Icon fontSize="small" />
              <span>{label}</span>
            </button>
          ))}
        </div>

        {showDirectorySetupHint ? (
          <p className="mobile-more-sheet__hint" role="status">
            {zh(
              '您还没有添加目录，请点击 “设置” 在 “目录管理” 内添加。',
              'No directories yet. Open Settings → Directory Management to add one.'
            )}
          </p>
        ) : null}
      </Drawer>
    </>
  )
}
