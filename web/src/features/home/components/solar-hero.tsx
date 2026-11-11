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
import { Link } from '@tanstack/react-router'
import {
  ArrowDown,
  ArrowUpRight,
  Boxes,
  Orbit,
  Pause,
  Play,
  RotateCcw,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import { lazy, Suspense, useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useSystemConfig } from '@/hooks/use-system-config'

import type { SolarBackgroundHandle } from './solar-background'

import '@/styles/solar-home.css'

const SolarBackground = lazy(() => import('./solar-background'))
const BODY_NAMES = [
  'Sun',
  'Mercury',
  'Venus',
  'Earth',
  'Mars',
  'Jupiter',
  'Saturn',
  'Uranus',
  'Neptune',
] as const

export function SolarHero(props: { isAuthenticated: boolean }) {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const scene = useRef<SolarBackgroundHandle>(null)
  const [ready, setReady] = useState(false)
  const [unavailable, setUnavailable] = useState(false)
  const [paused, setPaused] = useState(false)
  const [selectedBody, setSelectedBody] = useState<number | null>(null)
  const [pickerOpen, setPickerOpen] = useState(false)
  const onReady = useCallback(() => setReady(true), [])
  const onUnavailable = useCallback(() => setUnavailable(true), [])
  return (
    <section className='solar-hero' aria-label={t('Solar system')}>
      <img
        className='solar-sky'
        src='/textures/solar-system/galaxy.jpg'
        alt=''
        aria-hidden='true'
      />
      <div className='solar-stage' data-ready={ready || undefined}>
        <Suspense fallback={null}>
          <SolarBackground
            ref={scene}
            paused={paused}
            selectedBody={selectedBody}
            onReady={onReady}
            onUnavailable={onUnavailable}
          />
        </Suspense>
      </div>
      <div className='solar-shade' aria-hidden='true' />
      <div className='solar-copy'>
        <h1>{systemName || 'New API'}</h1>
        <p>{t('Every model. One connection.')}</p>
      </div>
      <div className='solar-actions'>
        <Button
          render={
            <Link to={props.isAuthenticated ? '/playground' : '/sign-in'} />
          }
          className='solar-action-primary'
        >
          {t('Open playground')}
          <ArrowUpRight aria-hidden='true' />
        </Button>
        <Button
          render={<Link to='/pricing' />}
          variant='outline'
          className='solar-action-secondary'
        >
          {t('Explore models')}
          <Boxes aria-hidden='true' />
        </Button>
      </div>
      <div
        className='solar-view-controls'
        role='group'
        aria-label={t('Scene controls')}
      >
        <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
          <PopoverTrigger
            render={
              <Button variant='ghost' aria-label={t('Choose a planet')} />
            }
          >
            <Orbit />
            {selectedBody === null
              ? t('Solar system')
              : t(BODY_NAMES[selectedBody + 1])}
          </PopoverTrigger>
          <PopoverContent align='end' className='w-72'>
            <div className='grid grid-cols-3 gap-1'>
              {BODY_NAMES.map((name, index) => (
                <Button
                  key={name}
                  variant='ghost'
                  aria-pressed={selectedBody === index - 1}
                  onClick={() => {
                    setSelectedBody(index - 1)
                    setPickerOpen(false)
                  }}
                >
                  {t(name)}
                </Button>
              ))}
            </div>
          </PopoverContent>
        </Popover>
        <Button
          variant='ghost'
          size='icon'
          aria-label={t('Zoom in')}
          onClick={() => scene.current?.zoom(0.8)}
        >
          <ZoomIn />
        </Button>
        <Button
          variant='ghost'
          size='icon'
          aria-label={t('Zoom out')}
          onClick={() => scene.current?.zoom(1.25)}
        >
          <ZoomOut />
        </Button>
        <Button
          variant='ghost'
          size='icon'
          aria-label={t('Reset view')}
          onClick={() => {
            setSelectedBody(null)
            scene.current?.reset()
          }}
        >
          <RotateCcw />
        </Button>
        <Button
          variant='ghost'
          size='icon'
          aria-label={paused ? t('Resume animation') : t('Pause animation')}
          aria-pressed={paused}
          onClick={() => setPaused(!paused)}
        >
          {paused ? <Play /> : <Pause />}
        </Button>
      </div>
      <div className='solar-caption'>
        <a href='#home-models' aria-label={t('Explore models')}>
          <ArrowDown className='size-4' />
          {t('Explore models')}
        </a>
        <span>
          {unavailable
            ? t('3D view is unavailable on this device.')
            : t('Drag to rotate. Ctrl + scroll to zoom.')}
        </span>
      </div>
      <div className='solar-texture-credit'>
        <a
          href='https://www.solarsystemscope.com/textures/'
          target='_blank'
          rel='noreferrer'
        >
          Solar System Scope / CC BY 4.0
        </a>
        <a
          href='https://maps.jpl.nasa.gov/tmaps/'
          target='_blank'
          rel='noreferrer'
        >
          NASA/JPL-Caltech/USGS
        </a>
      </div>
    </section>
  )
}
