/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  Activity,
  CheckCircle2,
  Clock,
  Eye,
  EyeOff,
  Globe2,
  KeyRound,
  Lock,
  RefreshCw,
  ShieldCheck,
  Zap,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  IconDiscord,
  IconGithub,
  IconGoogle,
  IconLinuxDo,
  IconTelegram,
  IconWeChat,
} from '@/assets/brand-icons'
import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/http-client'
import { cn } from '@/lib/utils'

export interface OAuthChannelItem {
  id: string
  name: string
  category: 'mainland' | 'global'
  enabled: boolean
  client_id: string
  has_secret: boolean
  redirect_uri: string
  scopes: string
  probe_latency_ms: number
  probe_status: 'ready' | 'pending' | 'error'
  last_probed_at?: string
}

export function OAuthChannelsSection() {
  const { t } = useTranslation()
  const [channels, setChannels] = useState<OAuthChannelItem[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [probingId, setProbingId] = useState<string | null>(null)
  const [isProbingAll, setIsProbingAll] = useState(false)

  // Modal editing state
  const [activeChannel, setActiveChannel] = useState<OAuthChannelItem | null>(
    null
  )
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [clientIdInput, setClientIdInput] = useState('')
  const [clientSecretInput, setClientSecretInput] = useState('')
  const [scopesInput, setScopesInput] = useState('')
  const [showSecret, setShowSecret] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const fetchChannels = useCallback(async () => {
    try {
      const res = await api.get<{
        success: boolean
        data: OAuthChannelItem[]
      }>('/api/oauth-channels/')
      if (res.data?.success && Array.isArray(res.data.data)) {
        setChannels(res.data.data)
      }
    } catch {
      toast.error(t('Failed to load OAuth channels'))
    } finally {
      setIsLoading(false)
    }
  }, [t])

  useEffect(() => {
    fetchChannels()
  }, [fetchChannels])

  const handleToggleEnable = async (channel: OAuthChannelItem) => {
    const nextState = !channel.enabled
    try {
      const res = await api.put<{ success: boolean; data: OAuthChannelItem }>(
        `/api/oauth-channels/${channel.id}`,
        { enabled: nextState }
      )
      if (res.data?.success) {
        setChannels((prev) =>
          prev.map((c) => (c.id === channel.id ? { ...c, enabled: nextState } : c))
        )
        toast.success(
          nextState
            ? t('{{name}} login enabled', { name: channel.name })
            : t('{{name}} login disabled', { name: channel.name })
        )
      }
    } catch {
      toast.error(t('Failed to update channel status'))
    }
  }

  const handleProbe = async (id: string) => {
    setProbingId(id)
    try {
      const res = await api.post<{
        success: boolean
        data: { id: string; probe_latency_ms: number; probe_status: string }
      }>(`/api/oauth-channels/${id}/probe`)
      if (res.data?.success) {
        const { probe_latency_ms, probe_status } = res.data.data
        setChannels((prev) =>
          prev.map((c) =>
            c.id === id
              ? {
                  ...c,
                  probe_latency_ms,
                  probe_status: probe_status as OAuthChannelItem['probe_status'],
                  last_probed_at: new Date().toISOString(),
                }
              : c
          )
        )
        if (probe_status === 'ready') {
          toast.success(
            t('Probe succeeded: {{latency}}ms', { latency: probe_latency_ms })
          )
        } else {
          toast.error(t('Probe failed / connection timed out'))
        }
      }
    } catch {
      toast.error(t('Failed to test connectivity'))
    } finally {
      setProbingId(null)
    }
  }

  const handleProbeAll = async () => {
    setIsProbingAll(true)
    try {
      for (const ch of channels) {
        await api.post(`/api/oauth-channels/${ch.id}/probe`)
      }
      await fetchChannels()
      toast.success(t('Full connectivity probe completed'))
    } catch {
      toast.error(t('Some probes encountered errors'))
    } finally {
      setIsProbingAll(false)
    }
  }

  const handleOpenConfig = (channel: OAuthChannelItem) => {
    setActiveChannel(channel)
    setClientIdInput(channel.client_id || '')
    setClientSecretInput('')
    setScopesInput(channel.scopes || '')
    setShowSecret(false)
    setIsDialogOpen(true)
  }

  const handleSaveConfig = async () => {
    if (!activeChannel) return
    setIsSaving(true)
    try {
      const payload: Record<string, any> = {
        client_id: clientIdInput.trim(),
        scopes: scopesInput.trim(),
      }
      if (clientSecretInput.trim()) {
        payload.client_secret = clientSecretInput.trim()
      }
      const res = await api.put<{ success: boolean; data: OAuthChannelItem }>(
        `/api/oauth-channels/${activeChannel.id}`,
        payload
      )
      if (res.data?.success) {
        toast.success(t('Credentials saved and applied hot-reloaded'))
        setIsDialogOpen(false)
        fetchChannels()
      }
    } catch {
      toast.error(t('Failed to save channel credentials'))
    } finally {
      setIsSaving(false)
    }
  }

  const mainlandChannels = useMemo(
    () => channels.filter((c) => c.category === 'mainland'),
    [channels]
  )
  const globalChannels = useMemo(
    () => channels.filter((c) => c.category === 'global'),
    [channels]
  )

  const getChannelIcon = (id: string) => {
    switch (id) {
      case 'google':
        return <IconGoogle className='h-6 w-6' />
      case 'github':
        return <IconGithub className='h-6 w-6' />
      case 'discord':
        return <IconDiscord className='h-6 w-6 text-indigo-400' />
      case 'telegram':
        return <IconTelegram className='h-6 w-6 text-sky-400' />
      case 'linuxdo':
        return <IconLinuxDo className='h-6 w-6 text-amber-500' />
      case 'wechat':
        return <IconWeChat className='h-6 w-6 text-emerald-400' />
      default:
        return <Globe2 className='h-6 w-6 text-muted-foreground' />
    }
  }

  const redirectUri = activeChannel
    ? `${window.location.origin}/oauth/${activeChannel.id}`
    : ''

  const renderChannelCard = (channel: OAuthChannelItem) => {
    const isReady = channel.enabled && channel.client_id && channel.has_secret
    const isProbing = probingId === channel.id

    return (
      <div
        key={channel.id}
        className='border-border/60 bg-card/60 hover:border-border hover:bg-card/90 flex flex-col justify-between rounded-xl border p-5 shadow-xs transition-all'
      >
        <div>
          {/* Top row: Logo + Name + Status Badge */}
          <div className='flex items-start justify-between gap-3'>
            <div className='flex items-center gap-3'>
              <div className='bg-muted/70 border-border/50 flex h-11 w-11 items-center justify-center rounded-lg border'>
                {getChannelIcon(channel.id)}
              </div>
              <div>
                <h4 className='text-sm font-semibold tracking-tight'>
                  {channel.name}
                </h4>
                <p className='text-muted-foreground text-xs'>
                  {channel.id === 'google'
                    ? t('Google Identity / OIDC')
                    : channel.id === 'github'
                      ? t('GitHub Apps / OAuth 2.0')
                      : t('OAuth 2.0 Standard')}
                </p>
              </div>
            </div>

            {isReady ? (
              <Badge
                variant='outline'
                className='border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center gap-1 font-normal'
              >
                <span className='h-1.5 w-1.5 rounded-full bg-emerald-500' />
                {t('Ready')}
              </Badge>
            ) : (
              <Badge
                variant='outline'
                className='border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center gap-1 font-normal'
              >
                <span className='h-1.5 w-1.5 rounded-full bg-amber-500' />
                {t('Pending Credentials')}
              </Badge>
            )}
          </div>

          {/* Telemetry row */}
          <div className='bg-muted/30 border-border/40 mt-4 rounded-lg border p-3 text-xs'>
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground flex items-center gap-1.5'>
                <Zap className='h-3.5 w-3.5 text-blue-500' />
                {t('Probe Latency')}
              </span>
              {channel.probe_latency_ms > 0 ? (
                <span className='font-mono font-medium text-emerald-500'>
                  {channel.probe_latency_ms} ms
                </span>
              ) : channel.probe_status === 'error' ? (
                <span className='font-medium text-rose-500'>
                  {t('Timed Out / Error')}
                </span>
              ) : (
                <span className='text-muted-foreground/60'>{t('Untested')}</span>
              )}
            </div>

            <div className='mt-2 flex items-center justify-between'>
              <span className='text-muted-foreground flex items-center gap-1.5'>
                <Activity className='h-3.5 w-3.5 text-purple-500' />
                {t('Security & TLS')}
              </span>
              <span className='text-muted-foreground font-medium'>
                {channel.probe_status === 'ready'
                  ? t('TLS 1.3 Active')
                  : t('Awaiting probe')}
              </span>
            </div>

            <div className='mt-2 flex items-center justify-between border-t border-border/30 pt-2 text-[11px] text-muted-foreground/70'>
              <span className='flex items-center gap-1'>
                <Clock className='h-3 w-3' />
                {t('Last probed')}
              </span>
              <span>
                {channel.last_probed_at
                  ? new Date(channel.last_probed_at).toLocaleTimeString()
                  : t('Never')}
              </span>
            </div>
          </div>
        </div>

        {/* Bottom row: Switch & Action buttons */}
        <div className='mt-5 flex items-center justify-between border-t border-border/40 pt-4'>
          <div className='flex items-center gap-2'>
            <Switch
              checked={channel.enabled}
              onCheckedChange={() => handleToggleEnable(channel)}
              id={`switch-${channel.id}`}
            />
            <Label
              htmlFor={`switch-${channel.id}`}
              className='cursor-pointer text-xs font-medium'
            >
              {t('Enable Login')}
            </Label>
          </div>

          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              className='h-8 text-xs'
              onClick={() => handleOpenConfig(channel)}
            >
              <KeyRound className='mr-1.5 h-3.5 w-3.5' />
              {t('Credentials')}
            </Button>
            <Button
              variant='secondary'
              size='sm'
              className='h-8 text-xs'
              disabled={isProbing}
              onClick={() => handleProbe(channel.id)}
            >
              <RefreshCw
                className={cn('h-3.5 w-3.5', isProbing && 'animate-spin')}
              />
              <span className='ml-1.5 hidden sm:inline'>{t('Test')}</span>
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className='space-y-8'>
      {/* Top Banner & Global Action */}
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h3 className='text-lg font-semibold tracking-tight'>
            {t('Login Methods & Connectivity Management')}
          </h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Manage third-party login providers, encrypted credentials, and real-time connectivity probes'
            )}
          </p>
        </div>
        <Button
          variant='outline'
          size='sm'
          disabled={isProbingAll || isLoading}
          onClick={handleProbeAll}
          className='gap-2'
        >
          <RefreshCw
            className={cn('h-4 w-4', isProbingAll && 'animate-spin')}
          />
          {t('Probe All Channels')}
        </Button>
      </div>

      {/* Global Channels */}
      <div className='space-y-4'>
        <div className='flex items-center gap-2 border-b border-border/40 pb-2'>
          <span className='text-base'>🌐</span>
          <h4 className='text-sm font-semibold tracking-tight'>
            {t('Global Channels (Google, GitHub, etc.)')}
          </h4>
        </div>
        <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-2'>
          {globalChannels.map(renderChannelCard)}
        </div>
      </div>

      {/* Mainland Channels */}
      <div className='space-y-4'>
        <div className='flex items-center gap-2 border-b border-border/40 pb-2'>
          <span className='text-base'>🇨🇳</span>
          <h4 className='text-sm font-semibold tracking-tight'>
            {t('Mainland Channels (WeChat, Lark, DingTalk, LINUX DO)')}
          </h4>
        </div>
        <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-2'>
          {mainlandChannels.map(renderChannelCard)}
        </div>
      </div>

      {/* Credential Management Dialog (Matching Screenshot 2) */}
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className='sm:max-w-[540px]'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2 text-base'>
              <KeyRound className='h-5 w-5 text-primary' />
              {t('Credentials Management & Setup - {{name}}', {
                name: activeChannel?.name,
              })}
            </DialogTitle>
            <DialogDescription className='text-xs'>
              {t(
                'Configure OAuth client credentials and verify callback endpoints.'
              )}
            </DialogDescription>
          </DialogHeader>

          {/* Security Banner */}
          <div className='border-primary/20 bg-primary/5 rounded-lg border p-3 text-xs'>
            <div className='flex items-start gap-2.5'>
              <ShieldCheck className='mt-0.5 h-4 w-4 shrink-0 text-primary' />
              <div>
                <p className='font-medium text-foreground'>
                  {t('Security & Compliance Standard')}
                </p>
                <p className='mt-0.5 text-muted-foreground leading-relaxed'>
                  {t(
                    'Client Secrets are stored with AES-256-GCM hardware-accelerated encryption at rest and will never be exposed in plaintext logs or frontend responses.'
                  )}
                </p>
              </div>
            </div>
          </div>

          <div className='space-y-4 py-2'>
            {/* Redirect URI */}
            <div className='space-y-1.5'>
              <Label className='text-xs text-muted-foreground'>
                {t('Authorized Redirect URI')}
              </Label>
              <div className='flex items-center gap-2'>
                <Input
                  readOnly
                  value={redirectUri}
                  className='bg-muted/40 font-mono text-xs'
                />
                <CopyButton text={redirectUri} />
              </div>
            </div>

            {/* Client ID / AppID */}
            <div className='space-y-1.5'>
              <Label htmlFor='client-id-input' className='text-xs'>
                {t('Client ID / AppID')}
              </Label>
              <Input
                id='client-id-input'
                placeholder={
                  activeChannel?.id === 'google'
                    ? '1029384756-xxxx.apps.googleusercontent.com'
                    : 'Ov23li...'
                }
                value={clientIdInput}
                onChange={(e) => setClientIdInput(e.target.value)}
                className='font-mono text-xs'
              />
            </div>

            {/* Client Secret / AppKey */}
            <div className='space-y-1.5'>
              <div className='flex items-center justify-between'>
                <Label htmlFor='client-secret-input' className='text-xs'>
                  {t('Client Secret / AppKey')}
                </Label>
                {activeChannel?.has_secret && (
                  <span className='text-[11px] text-emerald-500 font-medium'>
                    ✓ {t('Secret currently stored (encrypted)')}
                  </span>
                )}
              </div>
              <div className='relative'>
                <Input
                  id='client-secret-input'
                  type={showSecret ? 'text' : 'password'}
                  placeholder={
                    activeChannel?.has_secret
                      ? '•••••••••••••••• (Leave blank to keep existing)'
                      : t('Enter client secret')
                  }
                  value={clientSecretInput}
                  onChange={(e) => setClientSecretInput(e.target.value)}
                  className='font-mono pr-10 text-xs'
                />
                <button
                  type='button'
                  onClick={() => setShowSecret(!showSecret)}
                  className='text-muted-foreground hover:text-foreground absolute inset-y-0 right-0 flex items-center pr-3'
                >
                  {showSecret ? (
                    <EyeOff className='h-4 w-4' />
                  ) : (
                    <Eye className='h-4 w-4' />
                  )}
                </button>
              </div>
            </div>

            {/* Scopes */}
            <div className='space-y-1.5'>
              <Label htmlFor='scopes-input' className='text-xs'>
                {t('Authorized Scopes')}
              </Label>
              <Input
                id='scopes-input'
                placeholder='openid profile email'
                value={scopesInput}
                onChange={(e) => setScopesInput(e.target.value)}
                className='font-mono text-xs'
              />
            </div>
          </div>

          <DialogFooter className='gap-2 sm:gap-0'>
            <div className='mr-auto'>
              {activeChannel && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={probingId === activeChannel.id}
                  onClick={() => handleProbe(activeChannel.id)}
                  className='text-xs'
                >
                  <RefreshCw
                    className={cn(
                      'mr-1.5 h-3.5 w-3.5',
                      probingId === activeChannel.id && 'animate-spin'
                    )}
                  />
                  {t('Test Connectivity')}
                </Button>
              )}
            </div>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => setIsDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              size='sm'
              disabled={isSaving}
              onClick={handleSaveConfig}
            >
              {isSaving ? t('Saving...') : t('Save and Apply')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
