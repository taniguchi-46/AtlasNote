export function createRequestCounter(onChange: (count: number) => void) {
  let count = 0

  function begin() {
    count += 1
    onChange(count)
    let ended = false

    return () => {
      if (ended) return
      ended = true
      count = Math.max(0, count - 1)
      onChange(count)
    }
  }

  return { begin, getCount: () => count }
}
