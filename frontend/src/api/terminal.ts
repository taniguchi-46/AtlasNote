import { AckTerminal, ResizeTerminal, StartTerminal, StopTerminal, WriteTerminal } from '../../wailsjs/go/app/App'

export const terminalAPI = {
  start: StartTerminal,
  async write(id: string, data: string) {
    const bytes = new TextEncoder().encode(data)
    const decoder = new TextDecoder()
    for (let offset = 0; offset < bytes.length; offset += 8192) {
      const chunk = decoder.decode(bytes.subarray(offset, offset + 8192), { stream: offset + 8192 < bytes.length })
      if (chunk) await WriteTerminal(id, chunk)
    }
  },
  resize: ResizeTerminal,
  acknowledge: AckTerminal,
  stop: StopTerminal,
}
