export function createLatestRequestGuard() {
  let latestRequestId = 0

  return {
    begin() {
      // リクエスト開始時に一意のIDを発行し、非同期処理の完了後に現在の最新IDと一致するか確認する。
      // 一致しない場合は「後から別のリクエストが発行された（＝このリクエストは陳腐化した）」と判断でき、
      // 古いレスポンスでUIを上書きしてしまう不具合（レースコンディション）を防止できる。
      const requestId = ++latestRequestId
      return () => requestId === latestRequestId
    },
  }
}
