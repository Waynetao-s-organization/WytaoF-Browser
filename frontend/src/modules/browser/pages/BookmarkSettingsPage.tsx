import { useEffect, useMemo, useRef, useState } from 'react'
import { Plus, Trash2, RotateCcw, GripVertical, RefreshCw, Filter, ArrowUp, ArrowDown } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, Modal, toast } from '../../../shared/components'
import type { BrowserBookmark, BrowserGroupWithCount } from '../types'
import { fetchAllTags, fetchBookmarks, fetchBrowserProfiles, fetchGroups, resetBookmarks, saveBookmarks, syncBookmarksToProfiles } from '../api'
import { flattenGroups } from '../components/GroupSelector'

type ScopeMode = 'all' | 'custom'

interface ScopeDraft {
  index: number
  mode: ScopeMode
  tags: Set<string>
  groupIds: Set<string>
}

const getErrorMessage = (error: any, fallback: string) =>
  typeof error === 'string' && error.trim() ? error : (error?.message || fallback)

const chipClass = (active: boolean) =>
  `px-2.5 py-1 rounded-full text-xs font-medium transition-colors cursor-pointer ${
    active
      ? 'bg-[var(--color-accent)] text-white'
      : 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-subtle)]'
  }`

export function BookmarkSettingsPage() {
  const [items, setItems] = useState<BrowserBookmark[]>([])
  const [saving, setSaving] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [syncOpen, setSyncOpen] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [dragReadyIndex, setDragReadyIndex] = useState<number | null>(null)
  const dragIndexRef = useRef<number | null>(null)
  const [allTags, setAllTags] = useState<string[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [profileGroupIds, setProfileGroupIds] = useState<string[]>([])
  const [scopeDraft, setScopeDraft] = useState<ScopeDraft | null>(null)

  useEffect(() => {
    fetchBookmarks().then(setItems)
    fetchAllTags().then(setAllTags).catch(() => setAllTags([]))
    fetchGroups().then(setGroups).catch(() => setGroups([]))
    fetchBrowserProfiles()
      .then(profiles => setProfileGroupIds(Array.from(new Set(profiles.map(p => (p.groupId || '').trim()).filter(Boolean)))))
      .catch(() => setProfileGroupIds([]))
  }, [])

  const groupNameById = useMemo(() => new Map(groups.map(g => [g.groupId, g.groupName])), [groups])
  const groupLabel = (groupId: string) => groupNameById.get(groupId) || groupId

  const describeScope = (item: BrowserBookmark) => {
    const tags = item.openTags || []
    const groupIds = item.openGroupIds || []
    if (tags.length === 0 && groupIds.length === 0) return '所有实例'
    const parts: string[] = []
    if (tags.length > 0) parts.push(`标签：${tags.join('、')}`)
    if (groupIds.length > 0) parts.push(`分组：${groupIds.map(groupLabel).join('、')}`)
    return parts.join('；')
  }

  const handleChange = (index: number, field: 'name' | 'url', value: string) => {
    setItems(prev => prev.map((item, i) => i === index ? { ...item, [field]: value } : item))
  }

  const handleAdd = () => {
    setItems(prev => [...prev, { name: '', url: '', openOnStart: false, openTags: [], openGroupIds: [] }])
  }

  const handleDelete = (index: number) => {
    setItems(prev => prev.filter((_, i) => i !== index))
    setScopeDraft(prev => (prev && prev.index === index ? null : prev))
  }

  const handleMove = (index: number, delta: -1 | 1) => {
    setItems(prev => {
      const target = index + delta
      if (target < 0 || target >= prev.length) return prev
      const next = [...prev]
      const [moved] = next.splice(index, 1)
      next.splice(target, 0, moved)
      return next
    })
  }

  const handleOpenOnStartChange = (index: number, checked: boolean) => {
    setItems(prev => prev.map((item, i) => i === index ? { ...item, openOnStart: checked } : item))
  }

  const openScopeEditor = (index: number) => {
    const item = items[index]
    const tags = new Set(item.openTags || [])
    const groupIds = new Set(item.openGroupIds || [])
    setScopeDraft({ index, mode: tags.size + groupIds.size > 0 ? 'custom' : 'all', tags, groupIds })
  }

  const toggleScopeValue = (kind: 'tags' | 'groupIds', value: string) => {
    setScopeDraft(prev => {
      if (!prev) return prev
      const next = new Set(prev[kind])
      next.has(value) ? next.delete(value) : next.add(value)
      return { ...prev, mode: 'custom', [kind]: next }
    })
  }

  const handleScopeConfirm = () => {
    if (!scopeDraft) return
    const custom = scopeDraft.mode === 'custom'
    if (custom && scopeDraft.tags.size + scopeDraft.groupIds.size === 0) {
      toast.error('请至少选一个标签或分组，或者改成「所有实例」')
      return
    }
    setItems(prev => prev.map((item, i) => i === scopeDraft.index
      ? { ...item, openTags: custom ? Array.from(scopeDraft.tags) : [], openGroupIds: custom ? Array.from(scopeDraft.groupIds) : [] }
      : item))
    setScopeDraft(null)
  }

  const scopeTagOptions = useMemo(() => {
    const merged = new Set(allTags)
    scopeDraft?.tags.forEach(tag => merged.add(tag))
    return Array.from(merged).sort((a, b) => a.localeCompare(b, 'zh-CN'))
  }, [allTags, scopeDraft])

  const scopeGroupOptions = useMemo(() => {
    const options = flattenGroups(groups).map(g => ({ id: g.groupId, label: `${'　'.repeat(g.level)}${g.groupName}`, note: '' }))
    const known = new Set(options.map(o => o.id))
    profileGroupIds.forEach(id => {
      if (!known.has(id)) {
        known.add(id)
        options.push({ id, label: id, note: '实例上有这个分组，但分组列表里没有登记' })
      }
    })
    scopeDraft?.groupIds.forEach(id => {
      if (!known.has(id)) {
        known.add(id)
        options.push({ id, label: id, note: '这个分组已经不存在' })
      }
    })
    return options
  }, [groups, profileGroupIds, scopeDraft])

  const handleSave = async () => {
    const valid = items.filter(i => i.name.trim() && i.url.trim())
    if (valid.length !== items.length) {
      toast.error('存在空的名称或 URL，请填写完整后保存')
      return
    }
    setSaving(true)
    try {
      await saveBookmarks(items)
      const fresh = await fetchBookmarks()
      setItems(fresh)
      const result = await syncBookmarksToProfiles()
      const parts = ['书签已保存']
      if (result.synced > 0) parts.push(`已同步 ${result.synced} 个已有实例`)
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
    } catch (error: any) {
      toast.error(getErrorMessage(error, '保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    try {
      await resetBookmarks()
      const fresh = await fetchBookmarks()
      setItems(fresh)
      const result = await syncBookmarksToProfiles()
      const parts = ['已恢复默认书签']
      if (result.synced > 0) parts.push(`已同步 ${result.synced} 个已有实例`)
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个`)
      toast.success(parts.join('，'))
    } catch (error: any) {
      toast.error(getErrorMessage(error, '恢复默认书签失败'))
    }
  }

  const handleSync = async () => {
    setSyncing(true)
    try {
      const result = await syncBookmarksToProfiles()
      const parts = [`已同步 ${result.synced} 个实例`]
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
      setSyncOpen(false)
    } catch (error: any) {
      toast.error(getErrorMessage(error, '同步失败'))
    } finally {
      setSyncing(false)
    }
  }

  // 拖拽排序
  const handleDragStart = (e: React.DragEvent, index: number) => {
    if (dragReadyIndex !== index) {
      e.preventDefault()
      return
    }
    dragIndexRef.current = index
    setDragIndex(index)
    e.dataTransfer.effectAllowed = 'move'
    try {
      e.dataTransfer.setData('text/plain', String(index))
    } catch {}
  }
  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    const fromIndex = dragIndexRef.current
    if (fromIndex === null || fromIndex === index) return
    dragIndexRef.current = index
    setDragIndex(index)
    setItems(prev => {
      if (fromIndex < 0 || fromIndex >= prev.length || index < 0 || index >= prev.length) return prev
      const next = [...prev]
      const [moved] = next.splice(fromIndex, 1)
      next.splice(index, 0, moved)
      return next
    })
  }
  const handleDragEnd = () => {
    dragIndexRef.current = null
    setDragIndex(null)
    setDragReadyIndex(null)
  }

  const scopeBookmark = scopeDraft ? items[scopeDraft.index] : null

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">默认书签</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">保存后自动按当前顺序与列表同步到未运行实例的书签栏，实例里自己收藏的其它书签不受影响</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => setSyncOpen(true)} loading={syncing}>
            <RefreshCw className="w-4 h-4 mr-1.5" />
            手动同步
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setResetOpen(true)}>
            <RotateCcw className="w-4 h-4 mr-1.5" />
            恢复默认
          </Button>
          <Button size="sm" onClick={handleSave} loading={saving}>保存</Button>
        </div>
      </div>

      <Card title={`书签列表（${items.length} 项）`} subtitle="拖拽左侧手柄或点击右侧上下箭头可调整顺序；勾了「启动打开」后，可以点范围按钮限定只对某些标签或分组的实例打开">
        <div className="space-y-2">
          {items.map((item, index) => (
            <div
              key={index}
              draggable={dragReadyIndex === index || dragIndex === index}
              onDragStart={e => handleDragStart(e, index)}
              onDragOver={e => handleDragOver(e, index)}
              onDrop={e => e.preventDefault()}
              onDragEnd={handleDragEnd}
              className={`flex items-center gap-2 p-2.5 rounded-xl shadow-[var(--shadow-sm)] transition-all duration-150 ${
                dragIndex === index
                  ? 'bg-[var(--color-accent-muted)] ring-1 ring-[var(--color-border-strong)]'
                  : 'bg-[var(--color-bg-muted)] hover:bg-[var(--color-bg-subtle)]'
              }`}
            >
              <span
                onMouseEnter={() => setDragReadyIndex(index)}
                onMouseLeave={() => { if (dragIndex === null) setDragReadyIndex(null) }}
                onMouseDown={() => setDragReadyIndex(index)}
                title="按住拖拽调整顺序"
                className="p-1 rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] cursor-grab active:cursor-grabbing shrink-0"
              >
                <GripVertical className="w-4 h-4" />
              </span>
              <Input
                value={item.name}
                onChange={e => handleChange(index, 'name', e.target.value)}
                placeholder="名称，如 Google"
                className="w-36 shrink-0"
              />
              <Input
                value={item.url}
                onChange={e => handleChange(index, 'url', e.target.value)}
                placeholder="https://..."
                className="flex-1"
              />
              <label className="flex items-center gap-1.5 px-2 text-xs text-[var(--color-text-secondary)] whitespace-nowrap select-none">
                <input
                  type="checkbox"
                  checked={Boolean(item.openOnStart)}
                  onChange={e => handleOpenOnStartChange(index, e.target.checked)}
                  className="h-4 w-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)]"
                />
                启动打开
              </label>
              {item.openOnStart && (
                <button
                  type="button"
                  onClick={() => openScopeEditor(index)}
                  title={`启动打开的范围：${describeScope(item)}`}
                  className="flex items-center gap-1 max-w-[220px] shrink-0 px-2.5 py-1 rounded-lg text-xs border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)] transition-colors"
                >
                  <Filter className="w-3 h-3 shrink-0" />
                  <span className="truncate">{describeScope(item)}</span>
                </button>
              )}
              <div className="flex items-center gap-0.5 shrink-0">
                <button
                  type="button"
                  disabled={index === 0}
                  onClick={() => handleMove(index, -1)}
                  title="上移"
                  className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-surface)] disabled:opacity-30 disabled:pointer-events-none transition-colors"
                >
                  <ArrowUp className="w-3.5 h-3.5" />
                </button>
                <button
                  type="button"
                  disabled={index === items.length - 1}
                  onClick={() => handleMove(index, 1)}
                  title="下移"
                  className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-surface)] disabled:opacity-30 disabled:pointer-events-none transition-colors"
                >
                  <ArrowDown className="w-3.5 h-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => handleDelete(index)}
                  title="删除"
                  className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-red-500 hover:bg-red-50 transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}

          {items.length === 0 && (
            <p className="text-sm text-[var(--color-text-muted)] text-center py-6">
              暂无书签，点击下方按钮添加
            </p>
          )}
        </div>

        <button
          type="button"
          onClick={handleAdd}
          className="mt-3 w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-[var(--color-bg-muted)] text-sm text-[var(--color-text-primary)] shadow-[var(--shadow-sm)] hover:bg-[var(--color-bg-subtle)] transition-colors"
        >
          <Plus className="w-4 h-4" />
          添加书签
        </button>
      </Card>

      <Modal
        open={scopeDraft !== null}
        onClose={() => setScopeDraft(null)}
        title={`启动打开的范围${scopeBookmark?.name ? `：${scopeBookmark.name}` : ''}`}
        width="560px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setScopeDraft(null)}>取消</Button>
            <Button onClick={handleScopeConfirm}>确定</Button>
          </>
        }
      >
        {scopeDraft && (
          <div className="space-y-5">
            <div className="flex gap-2">
              <button type="button" className={chipClass(scopeDraft.mode === 'all')} onClick={() => setScopeDraft(prev => prev && { ...prev, mode: 'all' })}>
                所有实例
              </button>
              <button type="button" className={chipClass(scopeDraft.mode === 'custom')} onClick={() => setScopeDraft(prev => prev && { ...prev, mode: 'custom' })}>
                只对选中的标签或分组
              </button>
            </div>

            <div className={scopeDraft.mode === 'all' ? 'space-y-5 opacity-50' : 'space-y-5'}>
              <div>
                <p className="text-sm font-medium text-[var(--color-text-primary)] mb-2">标签</p>
                {scopeTagOptions.length === 0 ? (
                  <p className="text-xs text-[var(--color-text-muted)]">还没有任何标签，可以先在实例上加标签。</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {scopeTagOptions.map(tag => (
                      <button
                        key={tag}
                        type="button"
                        className={chipClass(scopeDraft.mode === 'custom' && scopeDraft.tags.has(tag))}
                        title={allTags.includes(tag) ? undefined : '现在已经没有实例用这个标签'}
                        onClick={() => toggleScopeValue('tags', tag)}
                      >
                        {tag}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div>
                <p className="text-sm font-medium text-[var(--color-text-primary)] mb-2">分组</p>
                {scopeGroupOptions.length === 0 ? (
                  <p className="text-xs text-[var(--color-text-muted)]">还没有任何分组。</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {scopeGroupOptions.map(option => (
                      <button
                        key={option.id}
                        type="button"
                        className={chipClass(scopeDraft.mode === 'custom' && scopeDraft.groupIds.has(option.id))}
                        title={option.note || undefined}
                        onClick={() => toggleScopeValue('groupIds', option.id)}
                      >
                        {option.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <p className="text-xs text-[var(--color-text-muted)]">
              实例带有任一选中的标签，或者在任一选中的分组里，启动时就会打开这个书签；其它实例不打开。改完记得点页面右上角的「保存」。
            </p>
          </div>
        )}
      </Modal>

      <ConfirmModal
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onConfirm={handleReset}
        title="恢复默认书签"
        content="将清除当前所有自定义书签，恢复为内置默认列表。确定继续？"
        confirmText="确定恢复"
        danger
      />

      <ConfirmModal
        open={syncOpen}
        onClose={() => setSyncOpen(false)}
        onConfirm={handleSync}
        title="手动同步已有实例"
        content="将按当前默认书签的顺序、名称与增删结果同步更新已有实例的书签栏，实例里自己收藏的其它书签不受影响。运行中的实例会跳过。"
        confirmText="开始同步"
      />
    </div>
  )
}
