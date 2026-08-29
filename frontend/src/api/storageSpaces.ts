import {
  CreateStorageSpace,
  ListStorageSpaces,
  SelectStorageSpace,
} from '../../wailsjs/go/main/App'

export type StorageSpace = {
  id: string
  name: string
  active: boolean
  legacy: boolean
  createdAt: string
}

export type StorageSpaceError = {
  code: string
  message: string
}

export type StorageSpaceListResult = {
  spaces: StorageSpace[]
  activeSpaceId: string
  error?: StorageSpaceError
}

export type StorageSpaceMutationResult = {
  space?: StorageSpace
  activeSpaceId?: string
  restartRequired: boolean
  error?: StorageSpaceError
}

export function listStorageSpaces(): Promise<StorageSpaceListResult> {
  return ListStorageSpaces()
}

export function createStorageSpace(name: string): Promise<StorageSpaceMutationResult> {
  return CreateStorageSpace({ name })
}

export function selectStorageSpace(id: string): Promise<StorageSpaceMutationResult> {
  return SelectStorageSpace({ id })
}
