export interface NotebookHierarchyItem {
  id: string
  parentId?: string | null
}

export function wouldCreateNotebookCycle(
  notebooks: readonly NotebookHierarchyItem[],
  id: string,
  parentId: string | null,
): boolean {
  if (parentId === null) return false
  if (parentId === id) return true

  const descendantIds = new Set<string>()
  const pendingParentIds = [id]

  while (pendingParentIds.length > 0) {
    const currentParentId = pendingParentIds.pop()
    if (!currentParentId) continue

    for (const notebook of notebooks) {
      if (notebook.parentId !== currentParentId || descendantIds.has(notebook.id)) continue
      if (notebook.id === parentId) return true

      descendantIds.add(notebook.id)
      pendingParentIds.push(notebook.id)
    }
  }

  return false
}
