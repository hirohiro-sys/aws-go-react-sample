# TODO

Nginx なしの Go API と React 画面、Postgres の TODO アプリです。画面のファイルは backend イメージに入り、Go が同じポートで配ります。

```
.
├── backend
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   └── main.go
├── db
│   └── password.txt
├── frontend
│   ├── index.html
│   ├── package.json
│   └── src
├── compose.yaml
└── README.md
```

## 起動

```shell
docker compose up --build
```

ブラウザで http://localhost:8000 を開きます。

TODO は Postgres のボリューム `db-data` に残ります。ボリュームごと消すときは `docker compose down -v` です。

## 画面だけ直すとき

backend が 8000 番で動いている状態で:

```shell
cd frontend
npm install
npm run dev
```

Vite は `/api` を http://localhost:8000 に渡します。
