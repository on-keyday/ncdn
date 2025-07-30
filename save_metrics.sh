# 1. ディレクトリをtarでアーカイブする
tar -cvf saved_metrics/metrics.tar ./metrics

# 2. tarアーカイブをzstdで圧縮し、50MBごとに分割保存する
zstd -c saved_metrics/metrics.tar | split -b 50M - saved_metrics/metrics.tar.zst.part_

rm saved_metrics/metrics.tar

# cat saved_metrics/metrics.tar.zst.part_* > saved_metrics/metrics.tar.zst
# zstd -d saved_metrics/metrics.tar.zst