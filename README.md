
# clangの用意の仕方
+ ubuntuのclangバージョン低すぎ問題解決法
```
curl -O https://api.llvm.org/llvm.sh
chmod +x llvm.sh
./llvm.sh 20
```

# 証明書の作り方(仮)
```
./gen_certs.sh 20250719194728 20250719194728 true
```
+ 実運用ではLets Encryptとかでやるになりそうかも
+ gen_certs.shは個人的に実験用に使ってたやつの流用
+ `20250719194728`はテキトーな日付からできた数値なので意味はない
+ ./ca/certs/20250719194728/に
  + root_ca.crt
  + intermediate_ca.crt
  + server.crt(intermediate_ca.crtを含む)

ができる
+ ./ca/private/20250719194728/に
  + root_ca.key
  + intermediate_ca.key
  + server.key

ができる

## サーバーが使うもの
+ server.crt(intermediate_ca.crtの情報はこれを読むだけでロードされるようになっている)
+ server.key

## クライアントが使うもの
+ root_ca.crt

# QUIC clientの試し方
```
go build -o ./secrets/qclient ./tool/quicclient
sudo env QUIC_GO_LOG_LEVEL=debug ip netns exec U ./secrets/qclient -serverAddress 192.0.2.10:8889 -requestCount 1 -rootCA /mnt/ncdn/ca/certs/20250719194728/root_ca.crt
```


# Linux kernel requirements

version 6.10 or later
ここらへんの関数群を使うため
+ https://docs.ebpf.io/linux/kfuncs/bpf_crypto_ctx_create/

# KVM

+ devcontainer上でeBPFプログラム走らせただけなのになぜかkernelがcrashしたのか何もかも巻き込んで再起動がかかるようになってしまったため別途KVM環境を構築
+ dpdkl4lb/virtsetup.sh の通りのを構築
+ ファイルマウント方法は https://ja.linux-console.net/?p=31022 参照
+ ただし普通にvirt-installに--filesystemってオプションがあったからそっちでもいけた説あり(未検証)
+ なおベースのままだとカーネルバージョンが6.8だったためinstall_new_kernel.shの通りカーネルのバージョンを上げるなどした
+ 蛇足: インターネット疎通しない現象にハマって時間をとかしていた
  + 原因はホストのLinux側にnftablesだけじゃなくてiptables(legacy)のforward禁止ルールがなぜか残っておりそれが悪さをしていた模様

