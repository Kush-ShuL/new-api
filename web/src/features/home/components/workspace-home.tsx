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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  ArrowUpRight,
  FlaskConical,
  HeartPulse,
  KeyRound,
  Trophy,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { ModelIdentity } from '@/components/model-identity'
import { Button } from '@/components/ui/button'
import { getRankings } from '@/features/rankings/api'
import { useStatus } from '@/hooks/use-status'
import { requireServerSuccess } from '@/lib/server-error-message'

import { SolarHero } from './solar-hero'

export function WorkspaceHome(props: { isAuthenticated: boolean }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [protocol, setProtocol] = useState<'responses' | 'messages'>(
    'responses'
  )
  const ranking = useQuery({
    queryKey: ['home-models'],
    queryFn: async () => requireServerSuccess(await getRankings('week')),
    staleTime: 300_000,
    retry: false,
    meta: { errorToast: false },
  })
  const base = String(status?.server_address || window.location.origin).replace(
    /\/$/,
    ''
  )
  const model = ranking.data?.data.models[0]?.model_name || 'your-model'
  const payload =
    protocol === 'responses'
      ? { model, input: 'Hello' }
      : {
          model,
          max_tokens: 256,
          messages: [{ role: 'user', content: 'Hello' }],
        }
  const headers =
    protocol === 'responses'
      ? ['-H "Authorization: Bearer $API_KEY"']
      : ['-H "x-api-key: $API_KEY"', '-H "anthropic-version: 2023-06-01"']
  const example = [
    `curl ${base}/v1/${protocol}`,
    ...headers,
    '-H "Content-Type: application/json"',
    `-d '${JSON.stringify(payload)}'`,
  ].join(' \\\n  ')

  return (
    <>
      <SolarHero isAuthenticated={props.isAuthenticated} />
      <main id='home-models' className='solar-workspace'>
        <div className='solar-workspace-heading'>
          <h2>{t('Make your first request')}</h2>
          <div className='solar-api-address'>
            <span>{t('API Base URL')}</span>
            <span className='truncate'>{base}/v1</span>
            <CopyButton value={`${base}/v1`} size='icon' />
          </div>
        </div>
        <div className='solar-workspace-grid'>
          <section
            className='solar-request'
            aria-label={t('Make your first request')}
          >
            <div
              className='solar-request-toolbar'
              role='group'
              aria-label={t('API protocol')}
            >
              <Button
                variant={protocol === 'responses' ? 'secondary' : 'ghost'}
                aria-pressed={protocol === 'responses'}
                onClick={() => setProtocol('responses')}
              >
                Responses
              </Button>
              <Button
                variant={protocol === 'messages' ? 'secondary' : 'ghost'}
                aria-pressed={protocol === 'messages'}
                onClick={() => setProtocol('messages')}
              >
                Messages
              </Button>
              <CopyButton value={example} size='icon' className='ml-auto' />
            </div>
            <pre>
              <code>{example}</code>
            </pre>
          </section>
          <section className='solar-popular'>
            <div className='solar-popular-heading'>
              <h2>{t('Popular this week')}</h2>
              <Link to='/rankings' aria-label={t('View rankings')}>
                <ArrowUpRight className='size-4' aria-hidden='true' />
              </Link>
            </div>
            {ranking.data?.data.models.slice(0, 3).map((entry) => (
              <Link
                key={entry.model_name}
                to='/pricing/$modelId'
                params={{ modelId: entry.model_name }}
                className='solar-popular-model'
              >
                <ModelIdentity
                  name={entry.model_name}
                  icon={entry.vendor_icon}
                  size={24}
                />
                <span className='solar-model-share'>
                  {(entry.share * 100).toFixed(1)}%
                </span>
              </Link>
            ))}
            {!ranking.data?.data.models.length && (
              <p className='text-muted-foreground text-sm'>
                {t('Explore models to get started')}
              </p>
            )}
          </section>
        </div>
        <nav className='solar-workspace-links' aria-label={t('Quick access')}>
          <Link to='/pricing'>
            <FlaskConical className='size-4' aria-hidden='true' />
            {t('Model Square')}
            <ArrowUpRight className='size-4' aria-hidden='true' />
          </Link>
          <Link to='/model-availability'>
            <HeartPulse className='size-4' aria-hidden='true' />
            {t('Model availability')}
            <ArrowUpRight className='size-4' aria-hidden='true' />
          </Link>
          <Link to='/rankings'>
            <Trophy className='size-4' aria-hidden='true' />
            {t('Rankings')}
            <ArrowUpRight className='size-4' aria-hidden='true' />
          </Link>
          <Link to='/keys'>
            <KeyRound className='size-4' aria-hidden='true' />
            {t('API Keys')}
            <ArrowUpRight className='size-4' aria-hidden='true' />
          </Link>
        </nav>
      </main>
    </>
  )
}
