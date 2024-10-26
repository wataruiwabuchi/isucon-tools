# 使い方
## 初期設定
inventory.yml にインスタンスのIPアドレスを記載する

## 各種コマンド

git での管理対象のファイルをサーバーから収集
```
ansible-playbook -i inventory.yml file_collecter.yml
```

初期設定
```
ansible-playbook -i inventory.yml webapp.yml
```

デプロイ
```
ansible-playbook -i inventory.yml deploy.yml
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