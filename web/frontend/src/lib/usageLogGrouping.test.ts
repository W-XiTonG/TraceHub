import { describe, expect, it } from 'vitest'
import { groupVideoUsageLogs, parseVideoJobId } from './usageLogGrouping'

describe('parseVideoJobId', () => {
  it('parses video_job prefix', () => {
    expect(parseVideoJobId('video_job:vid-123')).toBe('vid-123')
  })
  it('parses settle prefix', () => {
    expect(parseVideoJobId('video_settle_extra:abc')).toBe('abc')
  })
})

describe('groupVideoUsageLogs', () => {
  const model = 'doubao-seedance-1-5-pro-251215'

  it('groups create with poll rows when API order is newest-first (polls above create)', () => {
    const rows = [
      {
        id: 3,
        request_time: '2026-05-22T08:05:10Z',
        model: 'route:/v1/videos/:video_id',
        total_cost: 0,
        status_code: 404,
      },
      {
        id: 2,
        request_time: '2026-05-22T08:05:05Z',
        model,
        total_cost: 0,
        status_code: 403,
      },
      {
        id: 1,
        request_time: '2026-05-22T08:05:01Z',
        model,
        prompt_tokens: 8,
        total_cost: 4000,
        status_code: 200,
        error_message: 'video_job:vid-1',
      },
    ]
    const { groups, standalone } = groupVideoUsageLogs(rows)
    expect(standalone.length).toBe(0)
    expect(groups.length).toBe(1)
    expect(groups[0].primary.id).toBe(1)
    expect(groups[0].children.map((c) => c.id).sort()).toEqual([2, 3])
  })

  it('does not group chat calls with completion tokens as video', () => {
    const rows = [
      {
        id: 1,
        request_time: '2026-05-22T08:00:00Z',
        model: 'my-video-chat-model',
        prompt_tokens: 100,
        completion_tokens: 50,
        total_cost: 500,
        status_code: 200,
      },
      {
        id: 2,
        request_time: '2026-05-22T08:00:05Z',
        model: 'my-video-chat-model',
        prompt_tokens: 10,
        completion_tokens: 0,
        total_cost: 0,
        status_code: 500,
      },
    ]
    const { groups, standalone } = groupVideoUsageLogs(rows)
    expect(groups).toHaveLength(0)
    expect(standalone).toHaveLength(2)
  })

  it('does not leave poll rows as standalone when primary is below them in the list', () => {
    const rows = [
      { id: 10, request_time: '2026-05-22T15:07:50Z', model, total_cost: 0, status_code: 404 },
      { id: 11, request_time: '2026-05-22T15:07:45Z', model, total_cost: 0, status_code: 403 },
      {
        id: 12,
        request_time: '2026-05-22T15:07:40Z',
        model,
        prompt_tokens: 8,
        total_cost: 4000,
        status_code: 200,
      },
    ]
    const { groups, standalone } = groupVideoUsageLogs(rows)
    expect(standalone).toHaveLength(0)
    expect(groups).toHaveLength(1)
    expect(groups[0].children).toHaveLength(2)
  })
})
