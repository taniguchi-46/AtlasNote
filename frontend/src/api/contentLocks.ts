import {
  ChangeContentLockPassphrase as changeContentLockPassphraseRPC,
  DisableContentLock as disableContentLockRPC,
  EnableContentLock as enableContentLockRPC,
  GetContentLockStatus as getContentLockStatusRPC,
  ListContentLocks as listContentLocksRPC,
  ListRequiredContentLocks as listRequiredContentLocksRPC,
  ListStorageSpaceLockStatuses as listStorageSpaceLockStatusesRPC,
  LockContentNow as lockContentNowRPC,
  LockContentTargetsNow as lockContentTargetsNowRPC,
  UnlockContentLock as unlockContentLockRPC,
} from '../../wailsjs/go/main/App'

export type ContentLockTargetType = 'space' | 'notebook' | 'note'

export type ContentLockTarget = {
  type: ContentLockTargetType
  id: string
}

export type ContentLockError = {
  code: string
  message: string
  aiRecordCount?: number
}

export type ContentLock = {
  id: string
  targetType: ContentLockTargetType
  targetId: string
  targetName: string
  unlocked: boolean
  createdAt: string
  updatedAt: string
}

export type ContentLockStatus = {
  protected: boolean
  locked: boolean
  explicitLock: boolean
  source?: ContentLockTargetType
}

export type ContentLockListResult = {
  locks: ContentLock[]
  error?: ContentLockError
}

export type ContentLockMutationResult = {
  lock?: ContentLock
  removed: boolean
  unlocked: boolean
  restartRequired: boolean
  aiRecordCount?: number
  error?: ContentLockError
}

export type StorageSpaceLockStatus = {
  spaceId: string
  protected: boolean
  locked: boolean
  error?: ContentLockError
}

export type StorageSpaceLockStatusResult = {
  statuses: StorageSpaceLockStatus[]
  error?: ContentLockError
}

export function listContentLocks(): Promise<ContentLockListResult> {
  return listContentLocksRPC() as unknown as Promise<ContentLockListResult>
}

export function listRequiredContentLocks(target: ContentLockTarget): Promise<ContentLockListResult> {
  return listRequiredContentLocksRPC(target) as unknown as Promise<ContentLockListResult>
}

export function listStorageSpaceLockStatuses(): Promise<StorageSpaceLockStatusResult> {
  return listStorageSpaceLockStatusesRPC() as unknown as Promise<StorageSpaceLockStatusResult>
}

export function getContentLockStatus(target: ContentLockTarget): Promise<ContentLockStatus> {
  return getContentLockStatusRPC(target) as unknown as Promise<ContentLockStatus>
}

export function enableContentLock(input: {
  targetType: ContentLockTargetType
  targetId: string
  passphrase: string
  deleteAIRecords: boolean
}): Promise<ContentLockMutationResult> {
  return enableContentLockRPC(input) as unknown as Promise<ContentLockMutationResult>
}

export function unlockContentLock(input: {
  targetType: ContentLockTargetType
  targetId: string
  passphrase: string
}): Promise<ContentLockMutationResult> {
  return unlockContentLockRPC(input) as unknown as Promise<ContentLockMutationResult>
}

export function lockContentNow(target: ContentLockTarget): Promise<ContentLockMutationResult> {
  return lockContentNowRPC(target) as unknown as Promise<ContentLockMutationResult>
}

export function lockContentTargetsNow(targets: ContentLockTarget[]): Promise<ContentLockListResult> {
  return lockContentTargetsNowRPC(targets) as unknown as Promise<ContentLockListResult>
}

export function changeContentLockPassphrase(input: {
  targetType: ContentLockTargetType
  targetId: string
  currentPassphrase: string
  newPassphrase: string
}): Promise<ContentLockMutationResult> {
  return changeContentLockPassphraseRPC(input) as unknown as Promise<ContentLockMutationResult>
}

export function disableContentLock(input: {
  targetType: ContentLockTargetType
  targetId: string
  passphrase: string
}): Promise<ContentLockMutationResult> {
  return disableContentLockRPC(input) as unknown as Promise<ContentLockMutationResult>
}
