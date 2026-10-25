import { useCallback, useEffect, useState } from 'react'
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded'
import {
  fetchDownloaderProvider,
  fetchDownloaderProviders,
  testDownloaderProvider,
  updateDownloaderActiveProvider,
  updateDownloaderProvider,
} from '@/features/downloads/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const PROVIDER_META = {
  clouddrive2: {
    label: 'CloudDrive2',
    description: zh(
      '云盘离线下载，下载完成后回传到本地目录（默认）',
      'Cloud offline download, copied back to the local directory (default)'
    ),
  },
  aria2: {
    label: 'aria2',
    description: zh(
      '本地 RPC 下载器，直接写入下载目录',
      'Local RPC downloader writing straight into the download directory'
    ),
  },
  qbittorrent: {
    label: 'qBittorrent',
    description: zh(
      '本地 Web API 下载器，直接写入下载目录',
      'Local Web API downloader writing straight into the download directory'
    ),
  },
  xunlei: {
    label: zh('迅雷 NAS（内测）', 'Xunlei NAS (beta)'),
    description: zh(
      '服务器端迅雷内核，直接下载到媒体目录',
      'Server-side Xunlei kernel that downloads straight into the media directory'
    ),
  },
}

const defaultAddresses = {
  aria2: 'http://127.0.0.1:6800',
  qbittorrent: 'http://127.0.0.1:8080',
  xunlei: 'http://127.0.0.1:2345/webman/3rdparty/pan-xunlei-com/index.cgi',
}

export default function LocalDownloaderPicker({ activeProvider, onChanged, onToast }) {
  const [providers, setProviders] = useState([])
  const [active, setActive] = useState(activeProvider || '')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState('')
  const [configOpen, setConfigOpen] = useState(false)
  const [config, setConfig] = useState({
    address: '',
    username: '',
    apiToken: '',
    remoteFolder: '',
  })
  const [configLoaded, setConfigLoaded] = useState(false)

  const loadProviders = useCallback(async ({ quiet = false } = {}) => {
    if (!quiet) setLoading(true)
    try {
      const payload = await fetchDownloaderProviders()
      setProviders(Array.isArray(payload?.providers) ? payload.providers : [])
      setActive(payload?.active_provider || '')
      setError('')
    } catch (loadError) {
      if (!quiet) setError(getErrorMessage(loadError))
    } finally {
      if (!quiet) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadProviders()
  }, [loadProviders])

  useEffect(() => {
    setActive(activeProvider || '')
  }, [activeProvider])

  const loadConfig = useCallback(async (provider) => {
    if (!provider || provider === 'clouddrive2') return
    try {
      const payload = await fetchDownloaderProvider(provider)
      setConfig({
        address: payload?.address || defaultAddresses[provider] || '',
        username: payload?.username || '',
        apiToken: '',
        remoteFolder: payload?.remote_folder || (provider === 'xunlei' ? '/xunlei/AV' : ''),
      })
      setConfigLoaded(true)
    } catch {
      setConfigLoaded(true)
    }
  }, [])

  const selectProvider = async (provider) => {
    if (busy) return
    setBusy(`select-${provider}`)
    setError('')
    try {
      const payload = await updateDownloaderActiveProvider(provider)
      setActive(payload?.active_provider || provider)
      onChanged?.(payload)
      onToast?.(
        zh(
          `已切换到 ${PROVIDER_META[provider]?.label || provider}`,
          `Switched to ${PROVIDER_META[provider]?.label || provider}`
        )
      )
      await loadProviders({ quiet: true })
      if (provider === 'clouddrive2') setConfigOpen(false)
      else {
        setConfigOpen(true)
        await loadConfig(provider)
      }
    } catch (selectError) {
      setError(getErrorMessage(selectError))
    } finally {
      setBusy('')
    }
  }

  const saveConfig = async () => {
    if (!active || active === 'clouddrive2' || busy) return
    setBusy('config-save')
    setError('')
    try {
      await updateDownloaderProvider(active, {
        address: config.address,
        username: config.username,
        api_token: config.apiToken || undefined,
        remote_folder: config.remoteFolder,
      })
      setNotice(zh('下载器配置已保存', 'Downloader settings saved'))
      setConfig((current) => ({ ...current, apiToken: '' }))
      await loadProviders({ quiet: true })
    } catch (saveError) {
      setError(getErrorMessage(saveError))
    } finally {
      setBusy('')
    }
  }

  const testConfig = async () => {
    if (!active || active === 'clouddrive2' || busy) return
    setBusy('config-test')
    setError('')
    setNotice('')
    try {
      const result = await testDownloaderProvider(active, {
        address: config.address,
        username: config.username,
        api_token: config.apiToken || undefined,
        remote_folder: config.remoteFolder,
      })
      const version = result?.details?.version ? `（${result.details.version}）` : ''
      setNotice(zh(`连接成功${version}`, `Connected${version}`))
    } catch (testError) {
      setError(getErrorMessage(testError))
    } finally {
      setBusy('')
    }
  }

  const activeMeta = PROVIDER_META[active]
  const showConfig = active && active !== 'clouddrive2' && configOpen

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-gray-900">{zh('下载器', 'Downloader')}</h2>
          <p className="mt-0.5 text-xs text-gray-500">
            {zh(
              '自动检测本机可用的下载器，未选择时使用默认下载器。',
              'Detected automatically on this machine. The default downloader is used when none is selected.'
            )}
          </p>
        </div>
        <button
          type="button"
          onClick={() => loadProviders()}
          disabled={loading}
          aria-label={zh('重新检测下载器', 'Detect downloaders again')}
          title={zh('重新检测', 'Detect again')}
          className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-gray-200 bg-white text-gray-500 hover:bg-gray-50 disabled:opacity-50"
        >
          <RefreshRoundedIcon fontSize="inherit" className={loading ? 'animate-spin' : ''} />
        </button>
      </div>

      {error ? (
        <div role="alert" className="mt-3 rounded-lg bg-red-50 px-3 py-1.5 text-xs text-red-700">
          {error}
        </div>
      ) : null}
      {notice ? (
        <div
          role="status"
          className="mt-3 rounded-lg bg-green-50 px-3 py-1.5 text-xs text-green-700"
        >
          {notice}
        </div>
      ) : null}

      <div className="mt-3 space-y-2">
        {(loading && providers.length === 0
          ? [{ id: 'clouddrive2', kind: 'cloud', implemented: true, detected: true }]
          : providers
        ).map((provider) => {
          const meta = PROVIDER_META[provider.id] || { label: provider.id, description: '' }
          const selected = provider.id === active
          const selectable =
            provider.implemented && (provider.detected || provider.id === 'clouddrive2')
          return (
            <div
              key={provider.id}
              className={`flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2.5 ${
                selected ? 'border-blue-300 bg-blue-50/70' : 'border-gray-200 bg-white'
              }`}
            >
              <span
                className={`h-2 w-2 shrink-0 rounded-full ${
                  provider.detected ? 'bg-emerald-500' : 'bg-gray-300'
                }`}
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-semibold text-gray-900">{meta.label}</span>
                  {selected ? (
                    <span className="rounded-full bg-blue-600 px-1.5 py-0.5 text-[10px] font-semibold text-white">
                      {zh('当前使用', 'Active')}
                    </span>
                  ) : null}
                  {provider.detected ? (
                    <span className="text-[11px] text-emerald-600">
                      {provider.requires_auth
                        ? zh('已检测到，需要认证', 'Detected, auth required')
                        : zh('已检测到', 'Detected')}
                      {provider.version ? ` · ${provider.version}` : ''}
                    </span>
                  ) : (
                    <span className="text-[11px] text-gray-400">
                      {zh('未检测到', 'Not detected')}
                    </span>
                  )}
                </div>
                <div className="mt-0.5 truncate text-[11px] text-gray-500">
                  {meta.description}
                  {provider.address ? ` · ${provider.address}` : ''}
                </div>
              </div>
              {selected ? (
                provider.id !== 'clouddrive2' ? (
                  <button
                    type="button"
                    onClick={() => {
                      setConfigOpen((open) => !open)
                      if (!configLoaded) void loadConfig(provider.id)
                    }}
                    className="h-7 shrink-0 rounded-md border border-blue-200 bg-white px-2 text-xs font-medium text-blue-700"
                  >
                    {configOpen ? zh('收起配置', 'Hide settings') : zh('配置', 'Settings')}
                  </button>
                ) : null
              ) : (
                <button
                  type="button"
                  onClick={() => selectProvider(provider.id)}
                  disabled={!selectable || Boolean(busy)}
                  className="h-7 shrink-0 rounded-md bg-blue-600 px-2.5 text-xs font-medium text-white disabled:cursor-not-allowed disabled:bg-gray-200 disabled:text-gray-500"
                  title={
                    provider.detected
                      ? undefined
                      : zh(
                          '未检测到，仍可手动填写地址',
                          'Not detected; you can still enter the address manually'
                        )
                  }
                >
                  {busy === `select-${provider.id}`
                    ? zh('切换中…', 'Switching...')
                    : zh('使用', 'Use')}
                </button>
              )}
            </div>
          )
        })}
      </div>

      {showConfig ? (
        <div className="mt-3 rounded-lg border border-gray-200 bg-gray-50 p-3">
          <p className="text-xs text-gray-500">
            {zh(
              '留空时使用默认地址。下载器需与本程序运行在同一台机器，并能写入下载目录。',
              'Leave blank to use the default address. The downloader must run on this machine and be able to write into the download directory.'
            )}
          </p>
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1 text-xs font-medium text-gray-600">
              {zh('地址', 'Address')}
              <input
                value={config.address}
                onChange={(event) =>
                  setConfig((current) => ({ ...current, address: event.target.value }))
                }
                placeholder={defaultAddresses[active] || ''}
                className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-xs outline-none focus:border-blue-500"
              />
            </label>
            {active === 'qbittorrent' || active === 'xunlei' ? (
              <>
                <label className="flex flex-col gap-1 text-xs font-medium text-gray-600">
                  {zh('用户名', 'Username')}
                  <input
                    value={config.username}
                    onChange={(event) =>
                      setConfig((current) => ({ ...current, username: event.target.value }))
                    }
                    placeholder="admin"
                    autoComplete="username"
                    className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-xs outline-none focus:border-blue-500"
                  />
                </label>
                <label className="flex flex-col gap-1 text-xs font-medium text-gray-600">
                  {zh('密码', 'Password')}
                  <input
                    type="password"
                    value={config.apiToken}
                    onChange={(event) =>
                      setConfig((current) => ({ ...current, apiToken: event.target.value }))
                    }
                    placeholder={zh('留空则不修改', 'Leave blank to keep')}
                    autoComplete="current-password"
                    className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-xs outline-none focus:border-blue-500"
                  />
                </label>
              </>
            ) : (
              <label className="flex flex-col gap-1 text-xs font-medium text-gray-600">
                {zh('RPC 密钥', 'RPC secret')}
                <input
                  type="password"
                  value={config.apiToken}
                  onChange={(event) =>
                    setConfig((current) => ({ ...current, apiToken: event.target.value }))
                  }
                  placeholder={zh('未设置可留空', 'Leave blank if unset')}
                  autoComplete="off"
                  className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-xs outline-none focus:border-blue-500"
                />
              </label>
            )}
            <label className="flex flex-col gap-1 text-xs font-medium text-gray-600 sm:col-span-2">
              {active === 'xunlei'
                ? zh('迅雷下载根目录', 'Xunlei download root')
                : zh('下载器写入目录', 'Downloader directory')}
              <input
                value={config.remoteFolder}
                onChange={(event) =>
                  setConfig((current) => ({ ...current, remoteFolder: event.target.value }))
                }
                placeholder={
                  active === 'xunlei'
                    ? '/xunlei/AV'
                    : zh(
                        '留空则使用上面的下载目录',
                        'Leave blank to use the download directory above'
                      )
                }
                className="h-9 rounded-lg border border-gray-300 bg-white px-3 text-xs outline-none focus:border-blue-500"
              />
            </label>
          </div>
          <div className="mt-3 flex flex-wrap justify-end gap-2">
            <button
              type="button"
              onClick={testConfig}
              disabled={Boolean(busy)}
              className="h-8 rounded-lg border border-gray-200 bg-white px-3 text-xs font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-50"
            >
              {busy === 'config-test' ? zh('测试中…', 'Testing...') : zh('测试连接', 'Test')}
            </button>
            <button
              type="button"
              onClick={saveConfig}
              disabled={Boolean(busy)}
              className="h-8 rounded-lg bg-blue-600 px-3 text-xs font-semibold text-white disabled:opacity-50"
            >
              {busy === 'config-save' ? zh('保存中…', 'Saving...') : zh('保存下载器设置', 'Save')}
            </button>
          </div>
        </div>
      ) : null}

      {activeMeta && !loading && providers.length > 0 ? (
        <p className="mt-3 text-[11px] text-gray-400">
          {zh(`当前下载器：${activeMeta.label}`, `Current downloader: ${activeMeta.label}`)}
        </p>
      ) : null}
    </section>
  )
}
