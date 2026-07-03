import { GetStartupStatus } from '../../wailsjs/go/main/App'

export type StartupStatus = {
  ready: boolean
  message?: string
  dataDir?: string
}

export function getStartupStatus(): Promise<StartupStatus> {
  return GetStartupStatus()
}
