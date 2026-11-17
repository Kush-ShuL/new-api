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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useForm } from 'react-hook-form'
import { describe, expect, test } from 'vitest'

import { Form } from '@/components/ui/form'

import {
  buildSettingJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  type ChannelFormValues,
} from '../../lib/channel-form'
import { channelSchema } from '../../types'
import { ResponsesWebSocketSetting } from '../responses-websocket-setting'

function SettingsForm(props: {
  channelType: number
  disabled?: boolean
  onSave: (value: string) => void
}) {
  const form = useForm<ChannelFormValues>({
    defaultValues: { ...CHANNEL_FORM_DEFAULT_VALUES, type: props.channelType },
  })
  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSave(buildSettingJSON(values))
        )}
      >
        <ResponsesWebSocketSetting
          channelType={props.channelType}
          disabled={props.disabled}
        />
        <button type='submit'>Save</button>
      </form>
    </Form>
  )
}

describe('Responses WebSocket channel mode', () => {
  test.each(['pool', 'passthrough', 'http_bridge'] as const)(
    'preserves %s when saving and reopening a channel',
    (mode) => {
      const channel = channelSchema.parse({
        id: 1,
        name: 'Test',
        key: '',
        type: 1,
        status: 1,
        created_time: 0,
        test_time: 0,
        response_time: 0,
        balance_updated_time: 0,
        setting: JSON.stringify({ responses_websocket_mode: mode }),
      })
      const values = transformChannelToFormDefaults(channel)
      const saved = buildSettingJSON(values)
      expect(JSON.parse(saved).responses_websocket_mode).toBe(mode)
      expect(
        transformChannelToFormDefaults({ ...channel, setting: saved })
          .responses_websocket_mode
      ).toBe(mode)
    }
  )

  test.each([1, 14, 64])(
    'provider %s defaults to HTTP bridge and lets the user select each transport',
    async (channelType) => {
      const user = userEvent.setup()
      const saved: string[] = []
      render(
        <SettingsForm
          channelType={channelType}
          onSave={(value) => saved.push(value)}
        />
      )
      expect(screen.getByRole('radio', { name: 'HTTP bridge' })).toBeChecked()
      for (const [label, mode] of [
        ['Connection pool', 'pool'],
        ['Passthrough', 'passthrough'],
        ['HTTP bridge', 'http_bridge'],
      ]) {
        await user.click(screen.getByRole('radio', { name: label }))
        await user.click(screen.getByRole('button', { name: 'Save' }))
        expect(JSON.parse(saved.at(-1) || '{}').responses_websocket_mode).toBe(
          mode
        )
      }
    }
  )

  test('unsupported providers hide mode selection and omit it from saved settings', async () => {
    const user = userEvent.setup()
    const saved: string[] = []
    render(
      <SettingsForm channelType={2} onSave={(value) => saved.push(value)} />
    )
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(JSON.parse(saved[0]).responses_websocket_mode).toBeUndefined()
  })

  test('locked configuration prevents changing the selected transport', async () => {
    const user = userEvent.setup()
    render(<SettingsForm channelType={1} disabled onSave={() => undefined} />)
    const pool = screen.getByRole('radio', { name: 'Connection pool' })
    expect(pool).toHaveAttribute('aria-disabled', 'true')
    await user.click(pool)
    expect(screen.getByRole('radio', { name: 'HTTP bridge' })).toBeChecked()
  })
})
