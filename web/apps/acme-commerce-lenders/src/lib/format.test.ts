import { describe, expect, it } from 'vitest'
import { duration, money } from './format'

describe('money', () => {
  it('formats US cents as dollars with separators', () => {
    expect(money(0)).toBe('$0.00')
    expect(money(5)).toBe('$0.05')
    expect(money(11000)).toBe('$110.00')
    expect(money(3000000)).toBe('$30,000.00')
    expect(money(8108384)).toBe('$81,083.84')
    expect(money(123456789)).toBe('$1,234,567.89')
  })

  it('puts the minus sign before the dollar sign', () => {
    expect(money(-2500)).toBe('-$25.00')
  })
})

describe('duration', () => {
  it('uses milliseconds under a second and seconds above', () => {
    expect(duration(42.4)).toBe('42 ms')
    expect(duration(999)).toBe('999 ms')
    expect(duration(1300)).toBe('1.3 s')
  })
})
