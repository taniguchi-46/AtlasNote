import { GetAIArtifact, GetAIHistory, ListAIArtifactsPage, ListAIHistories } from '../../wailsjs/go/app/App'
import type { AIArtifact, AIHistory } from './ai'

function checkResponse<T>(value: T | undefined, error?: unknown): T {
  if (error) throw new Error('旧AIデータを読み込めませんでした。')
  if (!value) throw new Error('旧AIデータを読み込めませんでした。')
  return value
}

export async function listLegacyHistories(): Promise<AIHistory[]> {
  const response = await ListAIHistories()
  return checkResponse(response.items, response.error) as unknown as AIHistory[]
}

export async function getLegacyHistory(id: string): Promise<AIHistory> {
  const response = await GetAIHistory(id)
  return checkResponse(response.history, response.error) as unknown as AIHistory
}

export type LegacyArtifactKind = 'writing' | 'summary'
export type LegacyArtifactPage = { items: AIArtifact[]; hasNext: boolean }

export async function listLegacyArtifactsPage(kind: LegacyArtifactKind, offset: number): Promise<LegacyArtifactPage> {
  const response = await ListAIArtifactsPage(kind, offset)
  return {
    items: checkResponse(response.items, response.error) as unknown as AIArtifact[],
    hasNext: response.hasNext,
  }
}

export async function getLegacyArtifact(id: string): Promise<AIArtifact> {
  const response = await GetAIArtifact(id)
  return checkResponse(response.artifact, response.error) as unknown as AIArtifact
}
