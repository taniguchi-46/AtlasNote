export type ChangeReviewItem = {
  candidateId?: string
  noteId?: string
  noteTitle?: string
  before?: unknown
  after?: unknown
  reason?: string
  status?: string
}

export type ChangeReview = {
  operationId: string
  kind: string
  storageSpaceId: string
  targetIds: string[]
  expectedRevision?: number
  items: ChangeReviewItem[]
  impactCount: number
  createdAt: string
  expiresAt: string
  updatedAt: string
  state: string
  message?: string
}

export type ChangeResult = {
  operationId: string
  state: string
  items?: { candidateId: string; status: string; message?: string }[]
  message?: string
}

type ChangeBridge = {
  ListExternalChangeReviews(): Promise<ChangeReview[]>
  ApproveExternalChange(operationId: string): Promise<ChangeResult>
  RejectExternalChange(operationId: string): Promise<ChangeResult>
}

function bridge(): ChangeBridge {
  return (window as unknown as { go: { app: { App: ChangeBridge } } }).go.app.App
}

export function listExternalChangeReviews(): Promise<ChangeReview[]> {
  return bridge().ListExternalChangeReviews()
}

export function approveExternalChange(operationId: string): Promise<ChangeResult> {
  return bridge().ApproveExternalChange(operationId)
}

export function rejectExternalChange(operationId: string): Promise<ChangeResult> {
  return bridge().RejectExternalChange(operationId)
}
