import { useEffect, useState, type FormEvent } from "react"

type Todo = {
  id: number
  title: string
  completed: boolean
  createdAt: string
  updatedAt: string
}

const apiBase = import.meta.env.VITE_API_BASE || "/api"

async function request<T>(path: string, init?: RequestInit): Promise<T | undefined> {
  const response = await fetch(apiBase + path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  })
  if (response.status === 204) {
    return undefined
  }
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    const message = body && typeof body.error === "string" ? body.error : "リクエストに失敗しました"
    throw new Error(message)
  }
  return body as T
}

export function App() {
  const [todos, setTodos] = useState<Todo[]>([])
  const [title, setTitle] = useState("")
  const [error, setError] = useState("")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [draft, setDraft] = useState("")

  useEffect(() => {
    request<Todo[]>("/todos")
      .then((list) => setTodos(list ?? []))
      .catch((err: Error) => setError(err.message))
  }, [])

  async function addTodo(event: FormEvent) {
    event.preventDefault()
    const next = title.trim()
    if (!next) {
      setError("タイトルを入力してください")
      return
    }
    setError("")
    try {
      const created = await request<Todo>("/todos", {
        method: "POST",
        body: JSON.stringify({ title: next }),
      })
      if (created) {
        setTodos((current) => [created, ...current])
      }
      setTitle("")
    } catch (err) {
      setError(err instanceof Error ? err.message : "追加に失敗しました")
    }
  }

  async function toggleTodo(todo: Todo) {
    setError("")
    try {
      const updated = await request<Todo>("/todos/" + todo.id, {
        method: "PATCH",
        body: JSON.stringify({ completed: !todo.completed }),
      })
      if (updated) {
        setTodos((current) => current.map((item) => (item.id === updated.id ? updated : item)))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新に失敗しました")
    }
  }

  function startEdit(todo: Todo) {
    setEditingId(todo.id)
    setDraft(todo.title)
  }

  async function saveEdit(event: FormEvent) {
    event.preventDefault()
    if (editingId === null) {
      return
    }
    const next = draft.trim()
    if (!next) {
      setError("タイトルを入力してください")
      return
    }
    setError("")
    try {
      const updated = await request<Todo>("/todos/" + editingId, {
        method: "PATCH",
        body: JSON.stringify({ title: next }),
      })
      if (updated) {
        setTodos((current) => current.map((item) => (item.id === updated.id ? updated : item)))
      }
      setEditingId(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新に失敗しました")
    }
  }

  async function deleteTodo(id: number) {
    setError("")
    try {
      await request<void>("/todos/" + id, { method: "DELETE" })
      setTodos((current) => current.filter((item) => item.id !== id))
      if (editingId === id) {
        setEditingId(null)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "削除に失敗しました")
    }
  }

  return (
    <main>
      <h1>TODO</h1>
      <form onSubmit={addTodo}>
        <input
          type="text"
          value={title}
          placeholder="やることを入力"
          aria-label="新しい TODO"
          onChange={(event) => setTitle(event.target.value)}
        />
        <button className="primary" type="submit">
          追加
        </button>
      </form>
      {error ? <p className="error">{error}</p> : null}
      {todos.length === 0 ? <p className="empty">TODO はまだありません</p> : null}
      <ul>
        {todos.map((todo) => (
          <li key={todo.id} className={todo.completed ? "item completed" : "item"}>
            <input
              type="checkbox"
              checked={todo.completed}
              aria-label={todo.title + " を完了にする"}
              onChange={() => toggleTodo(todo)}
            />
            {editingId === todo.id ? (
              <form className="edit" onSubmit={saveEdit}>
                <input
                  type="text"
                  value={draft}
                  aria-label="タイトルを編集"
                  onChange={(event) => setDraft(event.target.value)}
                />
                <button className="primary" type="submit">
                  保存
                </button>
                <button type="button" onClick={() => setEditingId(null)}>
                  キャンセル
                </button>
              </form>
            ) : (
              <>
                <span className="title">{todo.title}</span>
                <button type="button" onClick={() => startEdit(todo)}>
                  編集
                </button>
                <button type="button" onClick={() => deleteTodo(todo.id)}>
                  削除
                </button>
              </>
            )}
          </li>
        ))}
      </ul>
    </main>
  )
}
