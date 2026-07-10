export function createLatestRequestGuard() {
  let latestRequestId = 0

  return {
    begin() {
      const requestId = ++latestRequestId
      return () => requestId === latestRequestId
    },
  }
}
