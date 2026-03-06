import { useEffect, useRef } from 'react'

/**
 * Calls `callback` immediately and then every `intervalMs` milliseconds.
 * Cleans up the interval on unmount or when dependencies change.
 * Passes the latest `callback` reference on each tick (avoids stale closure).
 */
export function usePolling(callback: () => void, intervalMs: number): void {
  const savedCallback = useRef(callback)

  // Keep ref up to date without restarting the interval
  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    // Call immediately on mount
    savedCallback.current()

    const id = setInterval(() => {
      savedCallback.current()
    }, intervalMs)

    return () => clearInterval(id)
  }, [intervalMs])
}
