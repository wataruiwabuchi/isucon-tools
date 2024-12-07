# isucon_tools

# 初動
1. サーバー立ち上げ
2. ssh 確認
3. サービス変更して初期ベンチ
4. inventory.yml の設定
5. ansible-playbook file_collector.yml
6. ansible-playbook webapp.yml
7. プロファイルが動作することを確認する
8. ベンチマーク
9. app 1 台、db 1 台に分散
10. ベンチマーク

- [pprotein の組み込み](https://github.com/narusejun/isucon13-final/commit/3210c87c83158010f27ca0d54e1071315d1b3fb1)

# チートシート

- [ ]  private でリポジトリ作成
- [ ]  db
  - [ ]  インデックスを貼る
  - [ ]  N+1 台 の解消
  - [ ]  不要な項目を取得しない
  - [ ]  Bulk インサート
  - [ ]  遅延インサート
  - [ ]  binlog の停止
  - [ ]  外部キー制約の削除
  - [ ]  明示的にトランザクションを貼る
- [ ]  静的ファイル
  - [ ]  db から剥がす
  - [ ]  nginx からの配信
  - [ ]  大きなファイルは streaming で扱う(メモリ節約)
  - [ ]  ファイルの逐次読み込みはしない(一括 or streaming)
  - [ ]  圧縮(負荷具合を見ながら圧縮率調整)
- [ ]  キャッシュ
  - [ ]  sync.map を使う(redis は rtt が大きいので非推奨、キャッシュ整合性のための内部 api を作成したほうが良い)
  - [ ]  nginx のキャッシュ
  - [ ]  静的ファイル
  - [ ]  client cache
  - [ ]  x-accel-redirect
  - [ ]  重い計算結果
  - [ ]  db に計算済み結果を格納するカラム作成
  - [ ]  関数呼び出しは single flight にする
- [ ]  MySQLのバッファープールサイズ
  - [ ]  ソートを含むクエリが `Using Temporary` になっている場合等
- [ ]  NGINXのlistenがhttp2になっているか（e.g., `listen 443 http2;`）
- [ ]  ログの出力をとめる
- [ ]  複数台構成
    - redis は使わずにアプリの internal な api とかでキャッシュの整合性をとるのが理想
- [ ]  ログの出力をとめる
- [ ]  再起動試験

## 行き詰まった時に見直すべきもの

- [ ]  マニュアルに得点に結びつくものがないか
  - [ ] キャンペーンなどの隠しパラメータ
- [ ]  nginx のプロファイル
- [ ]  mysql のプロファイル
- [ ]  アプリのプロファイル
- [ ]  CPU
    - [ ]  iowait
- [ ]  ネットワーク
- [ ]  ディスク
- [ ]  メモリ

## 事例集
- cp をしない(パスに注意)
- 余計な外部コマンド呼び出しをしない
