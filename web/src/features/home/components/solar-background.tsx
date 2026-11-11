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
import { useEffect, useImperativeHandle, useRef, type Ref } from 'react'

import { createSolarSystem } from '../lib/solar-system'

export type SolarBackgroundHandle = {
  reset: () => void
  zoom: (scale: number) => void
}

export default function SolarBackground(props: {
  ref?: Ref<SolarBackgroundHandle>
  paused: boolean
  selectedBody: number | null
  onReady: () => void
  onUnavailable: () => void
}) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const scene = useRef<ReturnType<typeof createSolarSystem> | null>(null)
  const onReady = props.onReady
  const onUnavailable = props.onUnavailable
  useEffect(() => {
    if (!canvas.current) return
    try {
      scene.current = createSolarSystem(canvas.current, onReady)
    } catch {
      onUnavailable()
    }
    return () => {
      scene.current?.dispose()
      scene.current = null
    }
  }, [onReady, onUnavailable])
  useEffect(() => {
    scene.current?.setPaused(props.paused)
  }, [props.paused])
  const selectedBody = props.selectedBody
  useEffect(() => {
    if (selectedBody === null) scene.current?.reset()
    else scene.current?.focus(selectedBody)
  }, [selectedBody])
  useImperativeHandle(
    props.ref,
    () => ({
      reset: () => scene.current?.reset(),
      zoom: (scale) => scene.current?.zoom(scale),
    }),
    []
  )
  return <canvas className='solar-canvas' ref={canvas} aria-hidden='true' />
}
