# 大量ノート性能計測

大量ノート時の起動復旧・検索・一覧取得は、`internal/note/performance_benchmark_test.go` のGoベンチマークで計測する。

## 実行方法

PowerShellでは、まず計測対象件数を指定する。未指定時は5,000件を使用する。

```powershell
$env:ATLASNOTE_BENCH_NOTES = "1000"
$env:GOCACHE = Join-Path $env:TEMP "atlasnote-gocache"
go test ./internal/note -run '^$' -bench 'BenchmarkLargeNote' -benchmem -count=3
```

本番想定の計測では、`ATLASNOTE_BENCH_NOTES`を`5000`以上に設定する。起動復旧は本文読み込みと検索索引再構築を含むため、確認時は必要に応じて`-benchtime=1x`を指定して実行時間を抑える。

## 計測範囲

- `BenchmarkLargeNoteRecovery`: `Service.Recover`の全件復旧。DBオープン、migration、Wails起動処理は含めない。
- `BenchmarkLargeNoteSearch`: `Service.Search`によるFTS5検索の1ページ目。
- `BenchmarkLargeNoteListPage`: `Service.ListPage`による一覧の先頭ページと深いページ。COUNT、LIMIT/OFFSET、Summary変換を含む。

ベンチマークfixtureの生成（DB、Markdown、検索索引）は計測時間に含めない。出力の`fixture-notes`は対象件数、`ns/op`と`allocs/op`は1操作あたりの値である。異なる端末・件数の結果を比較する場合は、同じGoバージョンと`ATLASNOTE_BENCH_NOTES`を使用する。

## 実測結果（2026-07-13）

同一端末で`ATLASNOTE_BENCH_NOTES=1000`、`-benchtime=1x`、`-count=1`として計測した。mtime差分検知の実装前後で、復旧処理は次のように改善した。

| 計測 | 実装前 | 実装後 |
| --- | ---: | ---: |
| 一覧 1ページ目 | 397.2µs/op | 408.9µs/op |
| 一覧 深いページ | 1.16ms/op | 1.14ms/op |
| FTS検索 | 11.8ms/op | 12.5ms/op |
| 起動復旧 | 5.59s/op | 23.8ms/op |

実装後の起動復旧は、索引状態とMarkdownのmtimeが一致する場合に本文読み込みと全件再構築を省略する。ファイル変更、mtime未保存、索引状態欠落時は従来どおり全件hash照合・再構築へフォールバックする。

## 本番想定基準値（5,000件）

Windows、AMD Ryzen 7 5700X、`ATLASNOTE_BENCH_NOTES=5000`、`-benchtime=1x`、`-count=1`で2026-07-13に計測した。

| 計測 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| 一覧 1ページ目 | 538,500 | 73,136 | 1,288 |
| 一覧 深いページ | 5,138,100 | 81,968 | 1,474 |
| FTS検索 | 58,373,400 | 52,392 | 934 |
| 起動復旧 | 118,449,600 | 13,961,632 | 270,555 |

この値を本番想定の初期基準値とし、Go・SQLite・OS・ノート件数が同じ条件で再計測する。ベンチマーク全体の実行時間は約117秒だった。
