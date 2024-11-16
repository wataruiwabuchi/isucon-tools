# 使い方
## 初期設定
inventory.yml にインスタンスのIPアドレスを記載する
ssh_config.sample をコピーして ssh_config を作成する、中身の ip はインスタンスのIPアドレスにする

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

# Tips
## 高速化
```
# ~/.ssh/config
ControlPersist 120
ControlMaster auto
ControlPath /tmp/.ssh-%u.%r@%h:%p
ServerAliveInterval 10
```