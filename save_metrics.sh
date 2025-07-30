# 1. ディレクトリをtarでアーカイブする
tar -cvf metrics.tar ./metrics

# 2. tarアーカイブをzstdで圧縮し、50MBごとに分割保存する
zstd -c metrics.tar | split -b 50M - metrics.tar.zst.part_

rm metrics.tar