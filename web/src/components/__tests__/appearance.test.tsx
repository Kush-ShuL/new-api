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
import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppearanceBackground } from '@/components/appearance-background'
import { AppearanceSettings } from '@/components/appearance-settings'
import { api } from '@/lib/api'
import { DEFAULT_APPEARANCE, type Appearance } from '@/lib/appearance'
import { mapStatusDataToConfig } from '@/lib/status-query'
import { useSystemConfigStore } from '@/stores/system-config-store'

beforeEach(() =>
  useSystemConfigStore.getState().setConfig({ appearance: DEFAULT_APPEARANCE })
)
afterEach(() => {
  useSystemConfigStore.getState().setConfig({ appearance: DEFAULT_APPEARANCE })
  localStorage.removeItem('newapi-appearance')
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function AppearanceControls() {
  const [value, setValue] = useState<Appearance>(DEFAULT_APPEARANCE)
  return <AppearanceSettings value={value} onChange={setValue} />
}

describe('custom background and glass UI', () => {
  it('uploads background images in chunks when crypto.randomUUID is unavailable', async () => {
    vi.stubGlobal('crypto', {
      getRandomValues: globalThis.crypto.getRandomValues.bind(
        globalThis.crypto
      ),
    })
    const post = vi
      .spyOn(api, 'post')
      .mockImplementation(async (_url, _data, config) => ({
        data: {
          success: true,
          data: {
            url:
              (config as { params: { offset: number } }).params.offset > 0
                ? '/api/appearance/background/image.png'
                : '',
          },
        },
      }))
    render(<AppearanceControls />)
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('switch', { name: 'Frosted glass UI (experimental)' })
    )
    const url = screen.getByLabelText('Background image URL')
    await user.type(url, 'https://example.com/image.webp')
    expect(url).toHaveValue('https://example.com/image.webp')
    await user.upload(
      screen.getByLabelText('Upload background image', { selector: 'input' }),
      new File([new Uint8Array(9 * 1024 * 1024)], 'background.png', {
        type: 'image/png',
      })
    )
    expect(post).toHaveBeenCalledTimes(2)
    const firstPart = post.mock.calls[0][2]?.params as {
      upload_id: string
      offset: number
      total: number
    }
    const secondPart = post.mock.calls[1][2]?.params as typeof firstPart
    expect(firstPart).toMatchObject({
      upload_id: expect.stringMatching(/^[a-f0-9-]{36}$/),
      offset: 0,
      total: 9 * 1024 * 1024,
    })
    expect(secondPart.upload_id).toBe(firstPart.upload_id)
    expect(secondPart.offset).toBe(8 * 1024 * 1024)
    expect(url).toHaveValue(
      new URL('/api/appearance/background/image.png', window.location.origin)
        .href
    )
  })
  it('allows uploading a background only after enabling experimental glass UI', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 200, 20)
    )
    render(<AppearanceControls />)
    expect(
      screen.getByRole('button', { name: 'Upload background image' })
    ).toBeDisabled()
    fireEvent.click(
      screen.getByRole('switch', { name: 'Frosted glass UI (experimental)' })
    )
    expect(
      screen.getByRole('button', { name: 'Upload background image' })
    ).toBeEnabled()
    expect(
      await screen.findByRole('slider', { name: 'Mask strength' })
    ).toBeEnabled()
    const blur = screen.getByRole('slider', { name: 'Blur strength' })
    expect(blur).toHaveAttribute('aria-valuenow', '20')
    fireEvent.keyDown(blur, { key: 'ArrowRight' })
    expect(blur).toHaveAttribute('aria-valuenow', '21')
  })

  it('uses the server appearance instead of a browser override and applies blur independently of panel opacity', () => {
    localStorage.setItem(
      'newapi-appearance',
      JSON.stringify({
        state: { glassEnabled: true, background: 'old-browser-image' },
      })
    )
    const appearance = {
      ...DEFAULT_APPEARANCE,
      background: 'data:image/png;base64,server',
      blurStrength: 32,
      surfaceOpacity: 60,
    }
    useSystemConfigStore
      .getState()
      .setConfig(mapStatusDataToConfig({ appearance }))
    const view = render(<AppearanceBackground />)
    expect(view.container.querySelector('img')).toBeNull()
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ appearance: { ...appearance, glassEnabled: true } })
    )
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      'data:image/png;base64,server'
    )
    expect(
      document.documentElement.style.getPropertyValue('--glass-blur')
    ).toBe('32px')
    expect(
      document.documentElement.style.getPropertyValue('--glass-opacity')
    ).toBe('60%')
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ appearance: { ...appearance, glassEnabled: false } })
    )
    expect(view.container.querySelector('img')).toBeNull()
  })
})
