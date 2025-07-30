+ 現時点の暫定。計測方法の定番とかいろいろよくわかってないし厳密さにはおそらく欠けているしもしかしたらデータ見間違いとかあるし計算方法違うとかあるし間違っているとか足りないところがある可能性あると思うので遠慮なく指摘を言っていただけますとありがたいです
+ あとデータセットからもっと別の比較とかしてみるのも面白いのかもしれない?(負荷環境の違い;無制限と1000のときとかあと10000のときとかも...)

+ NOTE: requests_per_secondのデータ数はdata frameに入れた時に自動でサイズが調整されて広がっているだけでリクエスト全体の平均(各ファイルデータ数/(全計測終了時刻-全計測開始時刻))を出しているので実質は10個であることに注意(10個*フィールド埋め100000分で多くなっているだけで平均したら一緒である)
+ シナリオが結構単純ではあるからなんとも言えない面はある
+ 元データはsaved_metricsディレクトリに圧縮されてあるので見たい場合はそれを参照されたし

環境
+ 貧弱な環境...ちゃんとテストするんだったらもっとちゃんと整備したほうがいい
```
CPU: N100 
OS: Ubuntu 24.04 with Linux kernel 6.15 on KVM on Arch Linux
CPU数: 2 (仮想化により,物理は4CPUあるはずだが...)
備考: いろんなプロセスが他に動いている
```

# 直列テストシナリオ
負荷が少ないため全体的に処理速度が速くまた有意な差は個々のリクエストメトリクスレベルでは確認できなかったが
RPSは有意に差があるのではないかという感じになっている(要検証: RPSの測り方,他プロセスとの干渉の影響)

# 並列テストシナリオ(無制限)
当初並列テストしようとして作ったはいいが制限とかしなかったためひたすらgoroutineを起動する感じでやったやつ
テスト環境のノイズの影響が強すぎたと思われ有意な差が計算上は出たものもあるが
以下2パターンで真逆の結果が出るなどしていたためあんまり意味のあるデータではないように思われる

# 並列テストシナリオ(1000並列)
並列数1000に制限して実行
直列実行シナリオと有意差があるかどうかについては同じであるが
やはり数値の押し上げについては環境要因(上記貧弱な環境構成)が大きなファクターを占めている気がするため本番環境でも検証が必要


# データ

Description: 直列テストシナリオ

```
Loading metrics from metrics/20250730063930/lb_exist_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730063930/lb_exist_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730065416/lb_not_exist_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Load Balancer Exists Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5370180090.0,
    "mean": 5370.18009,
    "median": 3977.0,
    "variance": 509679616.44659007,
    "std_dev": 22576.085055797208,
    "quantiles": {
      "0.25": 3595.0,
      "0.5": 3977.0,
      "0.75": 4515.0,
      "0.95": 7396.0,
      "0.99": 26166.070000000065
    },
    "skewness": 206.14491732750022,
    "kurtosis": 59018.284609670416,
    "min": 2265.0,
    "max": 8352812.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 25453177003.0,
    "mean": 25453.177003,
    "median": 17855.0,
    "variance": 1441943607.7704408,
    "std_dev": 37972.93256742809,
    "quantiles": {
      "0.25": 15972.0,
      "0.5": 17855.0,
      "0.75": 21553.0,
      "0.95": 53217.04999999993,
      "0.99": 176339.03000000003
    },
    "skewness": 52.94472012155293,
    "kurtosis": 8147.599129701928,
    "min": 10127.0,
    "max": 8618528.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 521957675382.0,
    "mean": 521957.675382,
    "median": 497493.0,
    "variance": 30954163494.7702,
    "std_dev": 175937.95353695063,
    "quantiles": {
      "0.25": 422218.0,
      "0.5": 497493.0,
      "0.75": 588365.25,
      "0.95": 766463.0499999999,
      "0.99": 1048743.3600000003
    },
    "skewness": 6.53192529357047,
    "kurtosis": 129.11616838736276,
    "min": 178921.0,
    "max": 10292950.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 548295906527.0,
    "mean": 548295.906527,
    "median": 521205.5,
    "variance": 33771687791.29381,
    "std_dev": 183770.7479205921,
    "quantiles": {
      "0.25": 443762.0,
      "0.5": 521205.5,
      "0.75": 616307.25,
      "0.95": 807053.0999999999,
      "0.99": 1114526.1900000002
    },
    "skewness": 6.3021534260683065,
    "kurtosis": 120.53518898959722,
    "min": 197599.0,
    "max": 10339599.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1799422173.5428426,
    "mean": 1799.4221735428425,
    "median": 1812.866757598575,
    "variance": 1694.9192711658209,
    "std_dev": 41.16939726502953,
    "quantiles": {
      "0.25": 1785.95972760828,
      "0.5": 1812.866757598575,
      "0.75": 1827.962514411428,
      "0.95": 1834.2261949034448,
      "0.99": 1834.2261949034448
    },
    "skewness": -1.7540134081957635,
    "kurtosis": 2.237585943641696,
    "min": 1689.412865169547,
    "max": 1834.2261949034448
  }
}
Load Balancer Does Not Exist Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5396642182.0,
    "mean": 5396.642182,
    "median": 3950.0,
    "variance": 531075215.22832555,
    "std_dev": 23045.069217260458,
    "quantiles": {
      "0.25": 3586.0,
      "0.5": 3950.0,
      "0.75": 4488.0,
      "0.95": 7626.0,
      "0.99": 26457.060000000056
    },
    "skewness": 191.49976705161524,
    "kurtosis": 50499.83719475198,
    "min": 2252.0,
    "max": 7055999.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 25952648797.0,
    "mean": 25952.648797,
    "median": 18060.0,
    "variance": 1574964328.491608,
    "std_dev": 39685.820244661794,
    "quantiles": {
      "0.25": 16142.0,
      "0.5": 18060.0,
      "0.75": 21916.0,
      "0.95": 55072.04999999993,
      "0.99": 177710.01
    },
    "skewness": 50.351659946556325,
    "kurtosis": 6934.092754986279,
    "min": 9959.0,
    "max": 7241030.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 525685840494.0,
    "mean": 525685.840494,
    "median": 498003.0,
    "variance": 38484845129.60845,
    "std_dev": 196175.54671673136,
    "quantiles": {
      "0.25": 422950.0,
      "0.5": 498003.0,
      "0.75": 589260.25,
      "0.95": 772470.0499999999,
      "0.99": 1114475.09
    },
    "skewness": 9.21182152460375,
    "kurtosis": 286.56368062885826,
    "min": 183379.0,
    "max": 17695874.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 552480395682.0,
    "mean": 552480.395682,
    "median": 522121.0,
    "variance": 41684086849.98506,
    "std_dev": 204166.8113332455,
    "quantiles": {
      "0.25": 444808.0,
      "0.5": 522121.0,
      "0.75": 617777.5,
      "0.95": 814190.1499999998,
      "0.99": 1184776.2500000002
    },
    "skewness": 8.748489658101578,
    "kurtosis": 256.80326976715935,
    "min": 200041.0,
    "max": 17752265.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1787199113.765714,
    "mean": 1787.199113765714,
    "median": 1816.4385912852429,
    "variance": 4852.176147978991,
    "std_dev": 69.65756346570694,
    "quantiles": {
      "0.25": 1783.5325794864289,
      "0.5": 1816.4385912852429,
      "0.75": 1823.8811483170302,
      "0.95": 1828.7946125758963,
      "0.99": 1828.7946125758963
    },
    "skewness": -2.236960048657244,
    "kurtosis": 3.6275295342279263,
    "min": 1588.8105790634645,
    "max": 1828.7946125758963
  }
}
H(0): LBが存在する場合でも、各メトリックの平均値はない場合と変わらない
H(1): LBが存在する場合、各メトリックの平均値はない場合に比べて有意に大きくなる。

Levene's test for connect_duration:
  Statistic: 1.1336243547684102
  P-value: 0.28700370428185673
connect_duration: p-value = 0.7939651505010661
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_sent_duration:
  Statistic: 36.89199650707203
  P-value: 1.2487981978333016e-09
request_sent_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 201.90787317536885
  P-value: 8.048884942589412e-46
response_first_byte_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for full_duration:
  Statistic: 207.07423179406734
  P-value: 6.005600642179178e-47
full_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_per_second:
  Statistic: 11293.702381243322
  P-value: 0.0
request_per_second: p-value = 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

```
Description: 並列テストシナリオ(無制限)1

```
Loading metrics from metrics/20250730082526/lb_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730082526/lb_not_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Load Balancer Exists Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 535833072519.0,
    "mean": 535833.072519,
    "median": 1638.0,
    "variance": 26925428756861.125,
    "std_dev": 5188971.840052818,
    "quantiles": {
      "0.25": 1494.0,
      "0.5": 1638.0,
      "0.75": 1828.0,
      "0.95": 2484.0,
      "0.99": 24085519.180000022
    },
    "skewness": 10.766187908579429,
    "kurtosis": 123.6118362628874,
    "min": 568.0,
    "max": 191926411.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4387341960115539.0,
    "mean": 4387341960.115539,
    "median": 4428867641.0,
    "variance": 2.673349232121924e+18,
    "std_dev": 1635037991.0332127,
    "quantiles": {
      "0.25": 3139282452.0,
      "0.5": 4428867641.0,
      "0.75": 5698315221.75,
      "0.95": 6972300663.2,
      "0.99": 7265621459.39
    },
    "skewness": -0.0982012985781839,
    "kurtosis": -0.9102083327504147,
    "min": 12947.0,
    "max": 7486526093.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4392337160120041.0,
    "mean": 4392337160.120041,
    "median": 4432488324.5,
    "variance": 2.6673659972103296e+18,
    "std_dev": 1633207273.1929433,
    "quantiles": {
      "0.25": 3143955172.75,
      "0.5": 4432488324.5,
      "0.75": 5703321853.5,
      "0.95": 6975443426.2,
      "0.99": 7269843818.75
    },
    "skewness": -0.09248389851286766,
    "kurtosis": -0.9232904841155918,
    "min": 12638753.0,
    "max": 7489376809.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4392430762213697.0,
    "mean": 4392430762.213697,
    "median": 4432591187.0,
    "variance": 2.667320573346136e+18,
    "std_dev": 1633193366.7959027,
    "quantiles": {
      "0.25": 3143965058.75,
      "0.5": 4432591187.0,
      "0.75": 5703483066.0,
      "0.95": 6975478482.2,
      "0.99": 7269851927.72
    },
    "skewness": -0.09245513757269118,
    "kurtosis": -0.9233156728838177,
    "min": 12651648.0,
    "max": 7489383706.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11257908048.085392,
    "mean": 11257.908048085392,
    "median": 11236.968725229166,
    "variance": 33693.86813224304,
    "std_dev": 183.55889554103075,
    "quantiles": {
      "0.25": 11098.083194561052,
      "0.5": 11236.968725229166,
      "0.75": 11447.916444863287,
      "0.95": 11531.079430110993,
      "0.99": 11531.079430110993
    },
    "skewness": 0.025030485206421316,
    "kurtosis": -1.4429742875306126,
    "min": 10990.893934466356,
    "max": 11531.079430110993
  }
}
Load Balancer Does Not Exist Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 489321863519.0,
    "mean": 489321.863519,
    "median": 1664.0,
    "variance": 32383241394521.84,
    "std_dev": 5690627.504460456,
    "quantiles": {
      "0.25": 1513.0,
      "0.5": 1664.0,
      "0.75": 1858.0,
      "0.95": 2521.0,
      "0.99": 633206.4800000899
    },
    "skewness": 13.266043653283438,
    "kurtosis": 190.8316117469704,
    "min": 562.0,
    "max": 206793630.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4447802238367517.0,
    "mean": 4447802238.367517,
    "median": 4486412560.5,
    "variance": 2.651465044113156e+18,
    "std_dev": 1628331982.156328,
    "quantiles": {
      "0.25": 3145969378.0,
      "0.5": 4486412560.5,
      "0.75": 5783102888.75,
      "0.95": 6979712614.8,
      "0.99": 7279914263.05
    },
    "skewness": -0.09934505936384153,
    "kurtosis": -0.9280242850213809,
    "min": 12691.0,
    "max": 7521338119.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4452764480166070.0,
    "mean": 4452764480.16607,
    "median": 4490280586.0,
    "variance": 2.645332744502195e+18,
    "std_dev": 1626447891.72669,
    "quantiles": {
      "0.25": 3150702635.0,
      "0.5": 4490280586.0,
      "0.75": 5786990090.0,
      "0.95": 6983072511.65,
      "0.99": 7283362271.2300005
    },
    "skewness": -0.09355760062921395,
    "kurtosis": -0.9420644094209343,
    "min": 11659254.0,
    "max": 7523611457.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4452863463229469.0,
    "mean": 4452863463.229469,
    "median": 4490336629.0,
    "variance": 2.645272130043692e+18,
    "std_dev": 1626429257.6204143,
    "quantiles": {
      "0.25": 3150731137.0,
      "0.5": 4490336629.0,
      "0.75": 5787172794.0,
      "0.95": 6983082393.15,
      "0.99": 7283601647.39
    },
    "skewness": -0.09351011331687936,
    "kurtosis": -0.9421707188328612,
    "min": 11682966.0,
    "max": 7523617383.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11194966603.296297,
    "mean": 11194.966603296298,
    "median": 11165.399242901596,
    "variance": 37429.64122095072,
    "std_dev": 193.4674164322011,
    "quantiles": {
      "0.25": 11081.373629344895,
      "0.5": 11165.399242901596,
      "0.75": 11268.163011105564,
      "0.95": 11640.054591856036,
      "0.99": 11640.054591856036
    },
    "skewness": 0.9367069721969561,
    "kurtosis": 0.48316467202683366,
    "min": 10904.972002029634,
    "max": 11640.054591856036
  }
}
H(0): LBが存在する場合でも、各メトリックの平均値はない場合と変わらない
H(1): LBが存在する場合、各メトリックの平均値はない場合に比べて有意に大きくなる。

Levene's test for connect_duration:
  Statistic: 36.505921964378416
  P-value: 1.5222986106130312e-09
connect_duration: p-value = 7.73262494249933e-10
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

Levene's test for request_sent_duration:
  Statistic: 24.32451343155557
  P-value: 8.140309654598224e-07
request_sent_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 24.22588537588324
  P-value: 8.567984507236516e-07
response_first_byte_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for full_duration:
  Statistic: 24.212087312963472
  P-value: 8.629586425763365e-07
full_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_per_second:
  Statistic: 36418.875218230685
  P-value: 0.0
request_per_second: p-value = 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

```
Description: 並列テストシナリオ(無制限)2

```
Loading metrics from metrics/20250730084603/lb_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730084603/lb_not_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Load Balancer Exists Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 273949073724.0,
    "mean": 273949.073724,
    "median": 1586.0,
    "variance": 13797650770206.996,
    "std_dev": 3714518.915042296,
    "quantiles": {
      "0.25": 1433.0,
      "0.5": 1586.0,
      "0.75": 1764.0,
      "0.95": 2310.0,
      "0.99": 14788.080000000075
    },
    "skewness": 15.110593855219584,
    "kurtosis": 237.84706269599494,
    "min": 553.0,
    "max": 158559542.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4504631847562704.0,
    "mean": 4504631847.562704,
    "median": 4494322586.5,
    "variance": 2.993200879581118e+18,
    "std_dev": 1730086957.231086,
    "quantiles": {
      "0.25": 3152062674.5,
      "0.5": 4494322586.5,
      "0.75": 5863218562.25,
      "0.95": 7147647501.9,
      "0.99": 8268210212.990001
    },
    "skewness": 0.024729430986274784,
    "kurtosis": -0.7284914051356473,
    "min": 11371.0,
    "max": 9233473891.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4509729254700237.0,
    "mean": 4509729254.700237,
    "median": 4498243572.5,
    "variance": 2.9873998170586895e+18,
    "std_dev": 1728409620.7377143,
    "quantiles": {
      "0.25": 3155412729.25,
      "0.5": 4498243572.5,
      "0.75": 5866730174.75,
      "0.95": 7150843591.4,
      "0.99": 8272215976.93
    },
    "skewness": 0.03007933751299823,
    "kurtosis": -0.738126135591592,
    "min": 33385185.0,
    "max": 9235164538.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4509828333390763.0,
    "mean": 4509828333.390763,
    "median": 4498302451.0,
    "variance": 2.9873638933044874e+18,
    "std_dev": 1728399228.5651157,
    "quantiles": {
      "0.25": 3155439549.5,
      "0.5": 4498302451.0,
      "0.75": 5866766639.75,
      "0.95": 7150857860.95,
      "0.99": 8272225329.38
    },
    "skewness": 0.03010648734237505,
    "kurtosis": -0.738133143741535,
    "min": 33391234.0,
    "max": 9235267128.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11057750356.049812,
    "mean": 11057.750356049812,
    "median": 11208.59269591216,
    "variance": 404701.5433517345,
    "std_dev": 636.161570162592,
    "quantiles": {
      "0.25": 11144.39073838721,
      "0.5": 11208.59269591216,
      "0.75": 11491.81317483518,
      "0.95": 11524.254349051917,
      "0.99": 11524.254349051917
    },
    "skewness": -2.175339928966859,
    "kurtosis": 3.5500899059154674,
    "min": 9253.534063461846,
    "max": 11524.254349051917
  }
}
Load Balancer Does Not Exist Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 418686133988.0,
    "mean": 418686.133988,
    "median": 1653.0,
    "variance": 18025885278633.92,
    "std_dev": 4245690.200501435,
    "quantiles": {
      "0.25": 1503.0,
      "0.5": 1653.0,
      "0.75": 1845.0,
      "0.95": 2468.0,
      "0.99": 14959931.020000001
    },
    "skewness": 11.279083501330247,
    "kurtosis": 142.8504624106013,
    "min": 566.0,
    "max": 265119580.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4465138890845067.0,
    "mean": 4465138890.845067,
    "median": 4553317910.5,
    "variance": 2.717435938131113e+18,
    "std_dev": 1648464721.5306468,
    "quantiles": {
      "0.25": 3125426516.0,
      "0.5": 4553317910.5,
      "0.75": 5811073910.0,
      "0.95": 6971676968.4,
      "0.99": 7230788917.02
    },
    "skewness": -0.16620335142848802,
    "kurtosis": -0.9585440904777704,
    "min": 13973.0,
    "max": 7505511744.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4470198604108239.0,
    "mean": 4470198604.108239,
    "median": 4557793325.5,
    "variance": 2.7107621655898875e+18,
    "std_dev": 1646439238.3534496,
    "quantiles": {
      "0.25": 3129363526.5,
      "0.5": 4557793325.5,
      "0.75": 5814581419.75,
      "0.95": 6976946895.7,
      "0.99": 7234686404.68
    },
    "skewness": -0.16061216202800496,
    "kurtosis": -0.9723658931436576,
    "min": 29462393.0,
    "max": 7506939315.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4470305984255514.0,
    "mean": 4470305984.255514,
    "median": 4557837068.5,
    "variance": 2.7106126456028e+18,
    "std_dev": 1646393830.6501274,
    "quantiles": {
      "0.25": 3129389321.75,
      "0.5": 4557837068.5,
      "0.75": 5814638196.0,
      "0.95": 6976956639.15,
      "0.99": 7234699853.67
    },
    "skewness": -0.16047623627233457,
    "kurtosis": -0.9727247852165295,
    "min": 29470669.0,
    "max": 7506960011.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11236399328.088867,
    "mean": 11236.399328088868,
    "median": 11259.048909448578,
    "variance": 24325.44833312704,
    "std_dev": 155.96617688821843,
    "quantiles": {
      "0.25": 11095.518209520664,
      "0.5": 11259.048909448578,
      "0.75": 11319.965736727707,
      "0.95": 11547.182770310023,
      "0.99": 11547.182770310023
    },
    "skewness": 0.11595691691723743,
    "kurtosis": -0.2878815572849742,
    "min": 10968.08845631723,
    "max": 11547.182770310023
  }
}
H(0): LBが存在する場合でも、各メトリックの平均値はない場合と変わらない
H(1): LBが存在する場合、各メトリックの平均値はない場合に比べて有意に大きくなる。

Levene's test for connect_duration:
  Statistic: 657.8851457959863
  P-value: 4.5480514277269005e-145
connect_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_sent_duration:
  Statistic: 1772.7241620947855
  P-value: 0.0
request_sent_duration: p-value = 1.1962431737871953e-61
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

Levene's test for response_first_byte_duration:
  Statistic: 1793.4434231789837
  P-value: 0.0
response_first_byte_duration: p-value = 6.817367162438619e-62
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

Levene's test for full_duration:
  Statistic: 1794.356759155295
  P-value: 0.0
full_duration: p-value = 7.19065606063123e-62
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

Levene's test for request_per_second:
  Statistic: 159489.58398861124
  P-value: 0.0
request_per_second: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

```
Description: 並列テストシナリオ(制限1000)

```
Loading metrics from metrics/20250730122515/lb_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_1-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_2-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_3-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_4-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_5-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_6-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_7-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_8-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_9-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250730122515/lb_not_exist_10-parallel.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Load Balancer Exists Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 160023203916.0,
    "mean": 160023.203916,
    "median": 1239.0,
    "variance": 3330710344416.7944,
    "std_dev": 1825023.3818822142,
    "quantiles": {
      "0.25": 1137.0,
      "0.5": 1239.0,
      "0.75": 1533.0,
      "0.95": 4294.0,
      "0.99": 323834.8500000185
    },
    "skewness": 13.220399469206422,
    "kurtosis": 191.6243658134732,
    "min": 758.0,
    "max": 40522654.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 79433087966317.0,
    "mean": 79433087.966317,
    "median": 71657989.5,
    "variance": 586551130931215.4,
    "std_dev": 24218817.70300143,
    "quantiles": {
      "0.25": 67586305.75,
      "0.5": 71657989.5,
      "0.75": 79192971.0,
      "0.95": 126093278.44999999,
      "0.99": 202915597.54000008
    },
    "skewness": 3.4100997695041624,
    "kurtosis": 15.395453981329103,
    "min": 11307.0,
    "max": 289783337.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 84082141794299.0,
    "mean": 84082141.794299,
    "median": 76076424.5,
    "variance": 613155331659192.4,
    "std_dev": 24761973.500898357,
    "quantiles": {
      "0.25": 71847882.5,
      "0.5": 76076424.5,
      "0.75": 83909939.25,
      "0.95": 131502857.74999997,
      "0.99": 211690730.23000005
    },
    "skewness": 3.465938075036457,
    "kurtosis": 15.674870490130862,
    "min": 23821119.0,
    "max": 297193072.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 84259478124585.0,
    "mean": 84259478.124585,
    "median": 76236420.0,
    "variance": 615066887004464.1,
    "std_dev": 24800542.070778698,
    "quantiles": {
      "0.25": 71974388.75,
      "0.5": 76236420.0,
      "0.75": 84135851.5,
      "0.95": 131780148.14999995,
      "0.99": 211848063.7
    },
    "skewness": 3.4533478750108872,
    "kurtosis": 15.572490775287799,
    "min": 23829861.0,
    "max": 297209636.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11805864640.131039,
    "mean": 11805.864640131038,
    "median": 11777.295307963228,
    "variance": 14567.576858127224,
    "std_dev": 120.69621724862476,
    "quantiles": {
      "0.25": 11707.44545530431,
      "0.5": 11777.295307963228,
      "0.75": 11901.782208573259,
      "0.95": 12014.946112365938,
      "0.99": 12014.946112365938
    },
    "skewness": 0.5582681120022696,
    "kurtosis": -1.0730959227877115,
    "min": 11659.010777239791,
    "max": 12014.946112365938
  }
}
Load Balancer Does Not Exist Metrics Statistics:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 182639002778.0,
    "mean": 182639.002778,
    "median": 1245.0,
    "variance": 4112573602179.284,
    "std_dev": 2027948.1261066033,
    "quantiles": {
      "0.25": 1141.0,
      "0.5": 1245.0,
      "0.75": 1546.0,
      "0.95": 4336.0,
      "0.99": 302819.8000000147
    },
    "skewness": 12.272313200811773,
    "kurtosis": 158.7123619033367,
    "min": 711.0,
    "max": 39552882.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 80140168669174.0,
    "mean": 80140168.669174,
    "median": 71758068.5,
    "variance": 600970942814776.0,
    "std_dev": 24514708.703445286,
    "quantiles": {
      "0.25": 67838416.5,
      "0.5": 71758068.5,
      "0.75": 79805675.5,
      "0.95": 130252819.25,
      "0.99": 196873454.85000005
    },
    "skewness": 3.092244399362508,
    "kurtosis": 12.349629464530047,
    "min": 18655.0,
    "max": 269252862.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 84829547550918.0,
    "mean": 84829547.550918,
    "median": 76273913.5,
    "variance": 628011748927893.2,
    "std_dev": 25060162.587818407,
    "quantiles": {
      "0.25": 72127532.5,
      "0.5": 76273913.5,
      "0.75": 84692832.25,
      "0.95": 135500643.2,
      "0.99": 204997437.77000004
    },
    "skewness": 3.1430905144240717,
    "kurtosis": 12.638574041653218,
    "min": 9803161.0,
    "max": 276376075.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 85009875854505.0,
    "mean": 85009875.854505,
    "median": 76412591.0,
    "variance": 630175615667238.2,
    "std_dev": 25103298.90008957,
    "quantiles": {
      "0.25": 72255028.0,
      "0.5": 76412591.0,
      "0.75": 84913318.5,
      "0.95": 135783225.2499999,
      "0.99": 205138307.44000006
    },
    "skewness": 3.129956918554241,
    "kurtosis": 12.530539156413857,
    "min": 9809337.0,
    "max": 276387217.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 11698728632.232058,
    "mean": 11698.728632232058,
    "median": 11700.86984434323,
    "variance": 14454.38247546084,
    "std_dev": 120.2263801146023,
    "quantiles": {
      "0.25": 11567.048163337824,
      "0.5": 11700.86984434323,
      "0.75": 11765.688663406001,
      "0.95": 11893.757347368602,
      "0.99": 11893.757347368602
    },
    "skewness": 0.06157573369644163,
    "kurtosis": -1.0090576217837008,
    "min": 11509.783143024822,
    "max": 11893.757347368602
  }
}
H(0): LBが存在する場合でも、各メトリックの平均値はない場合と変わらない
H(1): LBが存在する場合、各メトリックの平均値はない場合に比べて有意に大きくなる。

Levene's test for connect_duration:
  Statistic: 68.6877424805776
  P-value: 1.1542571820004053e-16
connect_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_sent_duration:
  Statistic: 202.2133415890698
  P-value: 6.9037606427747795e-46
request_sent_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 200.51166920540362
  P-value: 1.6232263778503832e-45
response_first_byte_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for full_duration:
  Statistic: 199.7735713874918
  P-value: 2.351970544679733e-45
full_duration: p-value = 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for request_per_second:
  Statistic: 606.7462681651879
  P-value: 5.97712803177974e-134
request_per_second: p-value = 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる

```
