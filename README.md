# isucon_tools

- [pprotein の組み込み](https://github.com/narusejun/isucon13-final/commit/3210c87c83158010f27ca0d54e1071315d1b3fb1)

- [ ]  private でリポジトリ作成
- [ ]  インデックスを貼る
    - [ ]  covering
    - [ ]  不要な項目を取得しない
- [ ]  Bulk インサート
- [ ]  遅延インサート
- [ ]  N+1 の解消
- [ ]  静的ファイルを db から剥がす
- [ ]  静的ファイルの nginx からの配信
- [ ]  キャッシュ
    - [ ]  静的ファイル
    - [ ]  client cache
    - [ ]  重い計算結果
    - [ ]  db に計算済み結果を格納するカラム作成
- [ ]  MySQLのバッファープールサイズ
    
    ソートを含むクエリが `Using Temporary` になっている場合等
    
- [ ]  NGINXのlistenがhttp2になっているか（e.g., `listen 443 http2;`）
- [ ]  ログの出力をとめる
- [ ]  複数台構成
    - redis は使わずにアプリの internal な api とかでキャッシュの整合性をとるのが理想
- [ ]  ログの出力をとめる
- [ ]  再起動試験

# 行き詰まった時に見直すべきもの

- [ ]  マニュアルに得点に結びつくものがないか
- [ ]  nginx のプロファイル
- [ ]  mysql のプロファイル
- [ ]  アプリのプロファイル
- [ ]  CPU
    - [ ]  iowait
- [ ]  ネットワーク
- [ ]  ディスク
- [ ]  メモリ