# 使い方
## 初期設定
- inventory.yml の各種設定を参加する isucon に合わせて変更
- ssh_config.sample をコピーして ssh_config を作成する、中身の ip はインスタンスのIPアドレスにする

## 各種コマンド

git での管理対象のファイルをサーバーから収集
```
ansible-playbook file_collecter.yml
```

初期設定
```
ansible-playbook webapp.yml
```

デプロイ
```
ansible-playbook deploy.yml
```
