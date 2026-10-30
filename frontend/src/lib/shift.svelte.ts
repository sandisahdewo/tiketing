import { App, model } from './api'

// The shift status of this PC's user, shared by the header and the cashier
// screen. offset is the server's clock minus this PC's, so a shift's schedule
// is judged by the server's time even when this PC's clock is off.
export const shift = $state<{ status: model.ShiftStatus | null; offset: number }>({
  status: null,
  offset: 0
})

// parseTime reads a stored time such as "2026-10-05 14:00:00" as local time.
export function parseTime(s: string): Date {
  const [date, time = '00:00:00'] = s.split(' ')
  const [y, m, d] = date.split('-').map(Number)
  const [h, min, sec] = time.split(':').map(Number)
  return new Date(y, m - 1, d, h, min, sec || 0)
}

export async function refreshShift(): Promise<model.ShiftStatus> {
  const status = await App.ShiftStatus()
  shift.offset = parseTime(status.serverTime).getTime() - Date.now()
  shift.status = status
  return status
}

export function clearShift() {
  shift.status = null
  shift.offset = 0
}

// isOverdue tells whether an open shift is past its scheduled end, by the
// server's clock.
export function isOverdue(s: model.Shift | null | undefined, now: Date): boolean {
  return !!s && !s.closedAt && now.getTime() + shift.offset >= parseTime(s.scheduledEndAt).getTime()
}

// shiftHours is "06:00–14:00", or "22:00–06:00 (+1 hari)" for a shift that
// ends the next day.
export function shiftHours(t: { startTime: string; endTime: string }): string {
  return `${t.startTime}–${t.endTime}${t.endTime <= t.startTime ? ' (+1 hari)' : ''}`
}
