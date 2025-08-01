Description: 直列テストシナリオ-QUICLBありなし

```
Loading metrics from metrics/20250801170034/lb_exist_quic_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
LBありQUIC:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5726128939.0,
    "mean": 5726.128939,
    "median": 3873.0,
    "variance": 1779420200.080266,
    "std_dev": 42183.174371783185,
    "quantiles": {
      "0.25": 3487.0,
      "0.5": 3873.0,
      "0.75": 4520.0,
      "0.95": 7729.0,
      "0.99": 34340.02000000002
    },
    "skewness": 507.6099316956001,
    "kurtosis": 358917.6726286706,
    "min": 2182.0,
    "max": 32403408.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 27987450092.0,
    "mean": 27987.450092,
    "median": 18230.0,
    "variance": 3140148980.4585896,
    "std_dev": 56037.032223865936,
    "quantiles": {
      "0.25": 16157.0,
      "0.5": 18230.0,
      "0.75": 23052.0,
      "0.95": 65765.0,
      "0.99": 207158.2100000002
    },
    "skewness": 223.5677457543015,
    "kurtosis": 117282.89643796642,
    "min": 10043.0,
    "max": 32545824.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 542784909083.0,
    "mean": 542784.909083,
    "median": 506262.0,
    "variance": 52762630772.64813,
    "std_dev": 229701.17712508165,
    "quantiles": {
      "0.25": 425474.75,
      "0.5": 506262.0,
      "0.75": 607524.0,
      "0.95": 870905.6499999991,
      "0.99": 1229049.2500000002
    },
    "skewness": 44.64079282980793,
    "kurtosis": 7948.559052604983,
    "min": 180803.0,
    "max": 57187756.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 572383076519.0,
    "mean": 572383.076519,
    "median": 531317.0,
    "variance": 58442814822.13652,
    "std_dev": 241749.48773913982,
    "quantiles": {
      "0.25": 448043.0,
      "0.5": 531317.0,
      "0.75": 637642.0,
      "0.95": 927773.2999999996,
      "0.99": 1331117.03
    },
    "skewness": 39.28039632393599,
    "kurtosis": 6571.115692638498,
    "min": 199609.0,
    "max": 57228899.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1741976095.0144503,
    "mean": 1741.9760950144503,
    "median": 1796.997291782277,
    "variance": 28096.76963448626,
    "std_dev": 167.6209104929521,
    "quantiles": {
      "0.25": 1779.8295653006562,
      "0.5": 1796.997291782277,
      "0.75": 1810.3420824209284,
      "0.95": 1820.0225227787193,
      "0.99": 1820.0225227787193
    },
    "skewness": -2.6327889291222197,
    "kurtosis": 5.005752648588706,
    "min": 1240.8259841071535,
    "max": 1820.0225227787193
  }
}
LBなしQUIC:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5487352508.0,
    "mean": 5487.352508,
    "median": 3892.0,
    "variance": 1640557886.065526,
    "std_dev": 40503.80088418278,
    "quantiles": {
      "0.25": 3513.0,
      "0.5": 3892.0,
      "0.75": 4458.0,
      "0.95": 7390.0,
      "0.99": 31687.02000000002
    },
    "skewness": 624.1152587225425,
    "kurtosis": 502521.69112135813,
    "min": 2162.0,
    "max": 34019851.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 25531334409.0,
    "mean": 25531.334409,
    "median": 17787.0,
    "variance": 2487321122.685782,
    "std_dev": 49873.0500639953,
    "quantiles": {
      "0.25": 15886.0,
      "0.5": 17787.0,
      "0.75": 21660.0,
      "0.95": 55181.04999999993,
      "0.99": 174086.08000000007
    },
    "skewness": 342.0331668489609,
    "kurtosis": 223080.42131388054,
    "min": 10181.0,
    "max": 34207212.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 506012440129.0,
    "mean": 506012.440129,
    "median": 482964.0,
    "variance": 51761601927.66716,
    "std_dev": 227511.7621743262,
    "quantiles": {
      "0.25": 406395.0,
      "0.5": 482964.0,
      "0.75": 573443.0,
      "0.95": 748275.0499999999,
      "0.99": 990882.0700000001
    },
    "skewness": 77.74420472964404,
    "kurtosis": 13025.210626229771,
    "min": 178110.0,
    "max": 56406622.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 534364412672.0,
    "mean": 534364.412672,
    "median": 509040.5,
    "variance": 54496340639.6199,
    "std_dev": 233444.51297818054,
    "quantiles": {
      "0.25": 429742.0,
      "0.5": 509040.5,
      "0.75": 603710.0,
      "0.95": 790316.2499999997,
      "0.99": 1055043.6000000006
    },
    "skewness": 72.89720082350145,
    "kurtosis": 11903.765940233829,
    "min": 192428.0,
    "max": 56483453.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1842570937.5592568,
    "mean": 1842.570937559257,
    "median": 1845.3950581032104,
    "variance": 656.1921172932089,
    "std_dev": 25.61624713523058,
    "quantiles": {
      "0.25": 1834.9800934019547,
      "0.5": 1845.3950581032104,
      "0.75": 1857.1616144751633,
      "0.95": 1883.6767803533417,
      "0.99": 1883.6767803533417
    },
    "skewness": -1.1679799712140722,
    "kurtosis": 1.7891437968760582,
    "min": 1777.757519243203,
    "max": 1883.6767803533417
  }
}
H(0): 「LBありQUIC」と「LBなしQUIC」に有意な差がない
H(1): 「LBありQUIC」のほうが「LBなしQUIC」よりも有意に大きい

Levene's test for connect_duration:
  Statistic: 19.838791835319558
  P-value: 8.425986140856597e-06
connect_duration: Statistic: 4.083003900969179 P-value: 2.2229502655407586e-05
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のconnect_durationの方が「LBなしQUIC」より大きい

Levene's test for request_sent_duration:
  Statistic: 849.2153903501446
  P-value: 1.1786182699244923e-186
request_sent_duration: Statistic: 32.74102112242955 P-value: 2.3574013398970937e-235
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のrequest_sent_durationの方が「LBなしQUIC」より大きい

Levene's test for response_first_byte_duration:
  Statistic: 3989.221911982736
  P-value: 0.0
response_first_byte_duration: Statistic: 113.7402807511161 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のresponse_first_byte_durationの方が「LBなしQUIC」より大きい

Levene's test for full_duration:
  Statistic: 4734.120977979706
  P-value: 0.0
full_duration: Statistic: 113.12916502931883 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のfull_durationの方が「LBなしQUIC」より大きい

Levene's test for request_per_second:
  Statistic: 95290.28507140855
  P-value: 0.0
request_per_second: Statistic: -593.2454580248464 P-value: 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

```
Description: 直列テストシナリオ-TCPLBありなし

```
Loading metrics from metrics/20250801170034/lb_exist_tcp_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
LBありTCP:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4272653799.0,
    "mean": 4272.653799,
    "median": 3154.0,
    "variance": 11283656139.599602,
    "std_dev": 106224.55525724549,
    "quantiles": {
      "0.25": 2810.0,
      "0.5": 3154.0,
      "0.75": 3589.0,
      "0.95": 6534.04999999993,
      "0.99": 27532.02000000002
    },
    "skewness": 560.4809680129175,
    "kurtosis": 319956.64527011267,
    "min": 1852.0,
    "max": 62538679.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 59999305521.0,
    "mean": 62481.05294393302,
    "median": 45564.0,
    "variance": 10282644422.875002,
    "std_dev": 101403.37481008707,
    "quantiles": {
      "0.25": 36730.0,
      "0.5": 45564.0,
      "0.75": 61697.0,
      "0.95": 179185.0,
      "0.99": 232255.46999999974
    },
    "skewness": 437.4151868785022,
    "kurtosis": 258357.1092593433,
    "min": 21189.0,
    "max": 60482308.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 197950101611.0,
    "mean": 197950.101611,
    "median": 184697.0,
    "variance": 568343518799.004,
    "std_dev": 753885.6138692421,
    "quantiles": {
      "0.25": 160872.0,
      "0.5": 184697.0,
      "0.75": 215788.0,
      "0.95": 276663.0,
      "0.99": 383425.03
    },
    "skewness": 266.5707251440751,
    "kurtosis": 72494.2413697993,
    "min": 80849.0,
    "max": 208675812.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 252325292952.0,
    "mean": 252325.292952,
    "median": 237466.5,
    "variance": 571288811954.4362,
    "std_dev": 755836.4981624241,
    "quantiles": {
      "0.25": 205169.0,
      "0.5": 237466.5,
      "0.75": 274046.0,
      "0.95": 355403.04999999993,
      "0.99": 499419.03
    },
    "skewness": 265.0810264748294,
    "kurtosis": 71953.7328518006,
    "min": 112249.0,
    "max": 208724185.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 3845859926.327252,
    "mean": 3845.8599263272517,
    "median": 3871.6279478393817,
    "variance": 15789.256747533369,
    "std_dev": 125.65530926918038,
    "quantiles": {
      "0.25": 3841.874146119459,
      "0.5": 3871.6279478393817,
      "0.75": 3934.167683747367,
      "0.95": 3957.4247580766882,
      "0.99": 3957.4247580766882
    },
    "skewness": -2.002268360724325,
    "kurtosis": 3.0962186496885944,
    "min": 3496.5568006561216,
    "max": 3957.4247580766882
  }
}
LBなしTCP:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4047997165.0,
    "mean": 4047.997165,
    "median": 3089.0,
    "variance": 1213427993.6576288,
    "std_dev": 34834.29335665687,
    "quantiles": {
      "0.25": 2744.0,
      "0.5": 3089.0,
      "0.75": 3537.0,
      "0.95": 6076.0,
      "0.99": 27977.0
    },
    "skewness": 741.2224015999036,
    "kurtosis": 644331.2118398049,
    "min": 1698.0,
    "max": 31171247.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 55797603250.0,
    "mean": 58127.22622254725,
    "median": 41815.0,
    "variance": 3525881721.5842886,
    "std_dev": 59379.13540617014,
    "quantiles": {
      "0.25": 33345.0,
      "0.5": 41815.0,
      "0.75": 58680.75,
      "0.95": 172187.8999999999,
      "0.99": 224114.79000000004
    },
    "skewness": 161.19774686143919,
    "kurtosis": 80665.01431292243,
    "min": 17652.0,
    "max": 31322782.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 190537743314.0,
    "mean": 190537.743314,
    "median": 180636.0,
    "variance": 6503469966.649817,
    "std_dev": 80644.09443133339,
    "quantiles": {
      "0.25": 157283.0,
      "0.5": 180636.0,
      "0.75": 211001.0,
      "0.95": 273161.0,
      "0.99": 349020.0800000001
    },
    "skewness": 80.97053897853613,
    "kurtosis": 27142.00109256241,
    "min": 78457.0,
    "max": 32727732.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 242468798621.0,
    "mean": 242468.798621,
    "median": 229938.0,
    "variance": 8525418130.5268955,
    "std_dev": 92333.19083908503,
    "quantiles": {
      "0.25": 197741.0,
      "0.5": 229938.0,
      "0.75": 268702.0,
      "0.95": 350952.09999999986,
      "0.99": 453472.1000000001
    },
    "skewness": 56.85610670761828,
    "kurtosis": 16178.623094526474,
    "min": 100624.0,
    "max": 32945896.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 4000964187.9392166,
    "mean": 4000.9641879392166,
    "median": 4044.028320181912,
    "variance": 25534.902057271574,
    "std_dev": 159.7964394386545,
    "quantiles": {
      "0.25": 4012.265495620111,
      "0.5": 4044.028320181912,
      "0.75": 4082.024360786621,
      "0.95": 4111.475757896079,
      "0.99": 4111.475757896079
    },
    "skewness": -2.4654703459896634,
    "kurtosis": 4.502772046183127,
    "min": 3531.3291933677824,
    "max": 4111.475757896079
  }
}
H(0): 「LBありTCP」と「LBなしTCP」に有意な差がない
H(1): 「LBありTCP」のほうが「LBなしTCP」よりも有意に大きい

Levene's test for connect_duration:
  Statistic: 2.0666227959180694
  P-value: 0.15055474454126105
connect_duration: Statistic: 2.0096244264330814 P-value: 0.022235587551700454
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありTCP」のconnect_durationの方が「LBなしTCP」より大きい

Levene's test for request_sent_duration:
  Statistic: nan
  P-value: nan
request_sent_duration: Statistic: nan P-value: nan
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 23.459412774495597
  P-value: 1.2758650459598227e-06
response_first_byte_duration: Statistic: 9.776429517952442 P-value: 7.12303713709318e-23
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありTCP」のresponse_first_byte_durationの方が「LBなしTCP」より大きい

Levene's test for full_duration:
  Statistic: 13.706752813666714
  P-value: 0.00021369050366996288
full_duration: Statistic: 12.944284258614266 P-value: 1.2743129403786573e-38
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありTCP」のfull_durationの方が「LBなしTCP」より大きい

Levene's test for request_per_second:
  Statistic: 738.0613567293217
  P-value: 1.6942006299222648e-162
request_per_second: Statistic: -762.9950486083811 P-value: 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

```
Description: 直列テストシナリオ-QUICとTCP LBなし

```
Loading metrics from metrics/20250801170034/lb_not_exist_quic_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_quic_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_not_exist_tcp_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
LBなしQUIC:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5487352508.0,
    "mean": 5487.352508,
    "median": 3892.0,
    "variance": 1640557886.065526,
    "std_dev": 40503.80088418278,
    "quantiles": {
      "0.25": 3513.0,
      "0.5": 3892.0,
      "0.75": 4458.0,
      "0.95": 7390.0,
      "0.99": 31687.02000000002
    },
    "skewness": 624.1152587225425,
    "kurtosis": 502521.69112135813,
    "min": 2162.0,
    "max": 34019851.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 25531334409.0,
    "mean": 25531.334409,
    "median": 17787.0,
    "variance": 2487321122.685782,
    "std_dev": 49873.0500639953,
    "quantiles": {
      "0.25": 15886.0,
      "0.5": 17787.0,
      "0.75": 21660.0,
      "0.95": 55181.04999999993,
      "0.99": 174086.08000000007
    },
    "skewness": 342.0331668489609,
    "kurtosis": 223080.42131388054,
    "min": 10181.0,
    "max": 34207212.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 506012440129.0,
    "mean": 506012.440129,
    "median": 482964.0,
    "variance": 51761601927.66716,
    "std_dev": 227511.7621743262,
    "quantiles": {
      "0.25": 406395.0,
      "0.5": 482964.0,
      "0.75": 573443.0,
      "0.95": 748275.0499999999,
      "0.99": 990882.0700000001
    },
    "skewness": 77.74420472964404,
    "kurtosis": 13025.210626229771,
    "min": 178110.0,
    "max": 56406622.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 534364412672.0,
    "mean": 534364.412672,
    "median": 509040.5,
    "variance": 54496340639.6199,
    "std_dev": 233444.51297818054,
    "quantiles": {
      "0.25": 429742.0,
      "0.5": 509040.5,
      "0.75": 603710.0,
      "0.95": 790316.2499999997,
      "0.99": 1055043.6000000006
    },
    "skewness": 72.89720082350145,
    "kurtosis": 11903.765940233829,
    "min": 192428.0,
    "max": 56483453.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1842570937.5592568,
    "mean": 1842.570937559257,
    "median": 1845.3950581032104,
    "variance": 656.1921172932089,
    "std_dev": 25.61624713523058,
    "quantiles": {
      "0.25": 1834.9800934019547,
      "0.5": 1845.3950581032104,
      "0.75": 1857.1616144751633,
      "0.95": 1883.6767803533417,
      "0.99": 1883.6767803533417
    },
    "skewness": -1.1679799712140722,
    "kurtosis": 1.7891437968760582,
    "min": 1777.757519243203,
    "max": 1883.6767803533417
  }
}
LBなしTCP:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4047997165.0,
    "mean": 4047.997165,
    "median": 3089.0,
    "variance": 1213427993.6576288,
    "std_dev": 34834.29335665687,
    "quantiles": {
      "0.25": 2744.0,
      "0.5": 3089.0,
      "0.75": 3537.0,
      "0.95": 6076.0,
      "0.99": 27977.0
    },
    "skewness": 741.2224015999036,
    "kurtosis": 644331.2118398049,
    "min": 1698.0,
    "max": 31171247.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 55797603250.0,
    "mean": 58127.22622254725,
    "median": 41815.0,
    "variance": 3525881721.5842886,
    "std_dev": 59379.13540617014,
    "quantiles": {
      "0.25": 33345.0,
      "0.5": 41815.0,
      "0.75": 58680.75,
      "0.95": 172187.8999999999,
      "0.99": 224114.79000000004
    },
    "skewness": 161.19774686143919,
    "kurtosis": 80665.01431292243,
    "min": 17652.0,
    "max": 31322782.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 190537743314.0,
    "mean": 190537.743314,
    "median": 180636.0,
    "variance": 6503469966.649817,
    "std_dev": 80644.09443133339,
    "quantiles": {
      "0.25": 157283.0,
      "0.5": 180636.0,
      "0.75": 211001.0,
      "0.95": 273161.0,
      "0.99": 349020.0800000001
    },
    "skewness": 80.97053897853613,
    "kurtosis": 27142.00109256241,
    "min": 78457.0,
    "max": 32727732.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 242468798621.0,
    "mean": 242468.798621,
    "median": 229938.0,
    "variance": 8525418130.5268955,
    "std_dev": 92333.19083908503,
    "quantiles": {
      "0.25": 197741.0,
      "0.5": 229938.0,
      "0.75": 268702.0,
      "0.95": 350952.09999999986,
      "0.99": 453472.1000000001
    },
    "skewness": 56.85610670761828,
    "kurtosis": 16178.623094526474,
    "min": 100624.0,
    "max": 32945896.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 4000964187.9392166,
    "mean": 4000.9641879392166,
    "median": 4044.028320181912,
    "variance": 25534.902057271574,
    "std_dev": 159.7964394386545,
    "quantiles": {
      "0.25": 4012.265495620111,
      "0.5": 4044.028320181912,
      "0.75": 4082.024360786621,
      "0.95": 4111.475757896079,
      "0.99": 4111.475757896079
    },
    "skewness": -2.4654703459896634,
    "kurtosis": 4.502772046183127,
    "min": 3531.3291933677824,
    "max": 4111.475757896079
  }
}
H(0): 「LBなしQUIC」と「LBなしTCP」に有意な差がない
H(1): 「LBなしQUIC」のほうが「LBなしTCP」よりも有意に大きい

Levene's test for connect_duration:
  Statistic: 161.810976906767
  P-value: 4.564823561517835e-37
connect_duration: Statistic: 26.94276196665975 P-value: 3.709474956614416e-160
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBなしQUIC」のconnect_durationの方が「LBなしTCP」より大きい

Levene's test for request_sent_duration:
  Statistic: nan
  P-value: nan
request_sent_duration: Statistic: nan P-value: nan
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 110357.71134802666
  P-value: 0.0
response_first_byte_duration: Statistic: 1306.9542589248385 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBなしQUIC」のresponse_first_byte_durationの方が「LBなしTCP」より大きい

Levene's test for full_duration:
  Statistic: 88901.45749457502
  P-value: 0.0
full_duration: Statistic: 1162.7391883145203 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBなしQUIC」のfull_durationの方が「LBなしTCP」より大きい

Levene's test for request_per_second:
  Statistic: 174482.5629382816
  P-value: 0.0
request_per_second: Statistic: -13336.864939370467 P-value: 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

```
Description: 直列テストシナリオ-QUICとTCP LBあり

```
Loading metrics from metrics/20250801170034/lb_exist_quic_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_quic_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_1.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_2.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_3.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_4.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_5.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_6.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_7.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_8.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_9.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
Loading metrics from metrics/20250801170034/lb_exist_tcp_10.json
Loaded 100000 metrics entries.
DataFrame shape: (100000, 9)
LBありQUIC:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 5726128939.0,
    "mean": 5726.128939,
    "median": 3873.0,
    "variance": 1779420200.080266,
    "std_dev": 42183.174371783185,
    "quantiles": {
      "0.25": 3487.0,
      "0.5": 3873.0,
      "0.75": 4520.0,
      "0.95": 7729.0,
      "0.99": 34340.02000000002
    },
    "skewness": 507.6099316956001,
    "kurtosis": 358917.6726286706,
    "min": 2182.0,
    "max": 32403408.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 27987450092.0,
    "mean": 27987.450092,
    "median": 18230.0,
    "variance": 3140148980.4585896,
    "std_dev": 56037.032223865936,
    "quantiles": {
      "0.25": 16157.0,
      "0.5": 18230.0,
      "0.75": 23052.0,
      "0.95": 65765.0,
      "0.99": 207158.2100000002
    },
    "skewness": 223.5677457543015,
    "kurtosis": 117282.89643796642,
    "min": 10043.0,
    "max": 32545824.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 542784909083.0,
    "mean": 542784.909083,
    "median": 506262.0,
    "variance": 52762630772.64813,
    "std_dev": 229701.17712508165,
    "quantiles": {
      "0.25": 425474.75,
      "0.5": 506262.0,
      "0.75": 607524.0,
      "0.95": 870905.6499999991,
      "0.99": 1229049.2500000002
    },
    "skewness": 44.64079282980793,
    "kurtosis": 7948.559052604983,
    "min": 180803.0,
    "max": 57187756.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 572383076519.0,
    "mean": 572383.076519,
    "median": 531317.0,
    "variance": 58442814822.13652,
    "std_dev": 241749.48773913982,
    "quantiles": {
      "0.25": 448043.0,
      "0.5": 531317.0,
      "0.75": 637642.0,
      "0.95": 927773.2999999996,
      "0.99": 1331117.03
    },
    "skewness": 39.28039632393599,
    "kurtosis": 6571.115692638498,
    "min": 199609.0,
    "max": 57228899.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 1741976095.0144503,
    "mean": 1741.9760950144503,
    "median": 1796.997291782277,
    "variance": 28096.76963448626,
    "std_dev": 167.6209104929521,
    "quantiles": {
      "0.25": 1779.8295653006562,
      "0.5": 1796.997291782277,
      "0.75": 1810.3420824209284,
      "0.95": 1820.0225227787193,
      "0.99": 1820.0225227787193
    },
    "skewness": -2.6327889291222197,
    "kurtosis": 5.005752648588706,
    "min": 1240.8259841071535,
    "max": 1820.0225227787193
  }
}
LBありTCP:
{
  "connect_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 4272653799.0,
    "mean": 4272.653799,
    "median": 3154.0,
    "variance": 11283656139.599602,
    "std_dev": 106224.55525724549,
    "quantiles": {
      "0.25": 2810.0,
      "0.5": 3154.0,
      "0.75": 3589.0,
      "0.95": 6534.04999999993,
      "0.99": 27532.02000000002
    },
    "skewness": 560.4809680129175,
    "kurtosis": 319956.64527011267,
    "min": 1852.0,
    "max": 62538679.0
  },
  "request_sent_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 59999305521.0,
    "mean": 62481.05294393302,
    "median": 45564.0,
    "variance": 10282644422.875002,
    "std_dev": 101403.37481008707,
    "quantiles": {
      "0.25": 36730.0,
      "0.5": 45564.0,
      "0.75": 61697.0,
      "0.95": 179185.0,
      "0.99": 232255.46999999974
    },
    "skewness": 437.4151868785022,
    "kurtosis": 258357.1092593433,
    "min": 21189.0,
    "max": 60482308.0
  },
  "response_first_byte_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 197950101611.0,
    "mean": 197950.101611,
    "median": 184697.0,
    "variance": 568343518799.004,
    "std_dev": 753885.6138692421,
    "quantiles": {
      "0.25": 160872.0,
      "0.5": 184697.0,
      "0.75": 215788.0,
      "0.95": 276663.0,
      "0.99": 383425.03
    },
    "skewness": 266.5707251440751,
    "kurtosis": 72494.2413697993,
    "min": 80849.0,
    "max": 208675812.0
  },
  "full_duration": {
    "unit": "nanoseconds",
    "data_count": 1000000,
    "sum": 252325292952.0,
    "mean": 252325.292952,
    "median": 237466.5,
    "variance": 571288811954.4362,
    "std_dev": 755836.4981624241,
    "quantiles": {
      "0.25": 205169.0,
      "0.5": 237466.5,
      "0.75": 274046.0,
      "0.95": 355403.04999999993,
      "0.99": 499419.03
    },
    "skewness": 265.0810264748294,
    "kurtosis": 71953.7328518006,
    "min": 112249.0,
    "max": 208724185.0
  },
  "request_per_second": {
    "unit": "requests/second",
    "data_count": 1000000,
    "sum": 3845859926.327252,
    "mean": 3845.8599263272517,
    "median": 3871.6279478393817,
    "variance": 15789.256747533369,
    "std_dev": 125.65530926918038,
    "quantiles": {
      "0.25": 3841.874146119459,
      "0.5": 3871.6279478393817,
      "0.75": 3934.167683747367,
      "0.95": 3957.4247580766882,
      "0.99": 3957.4247580766882
    },
    "skewness": -2.002268360724325,
    "kurtosis": 3.0962186496885944,
    "min": 3496.5568006561216,
    "max": 3957.4247580766882
  }
}
H(0): 「LBありQUIC」と「LBありTCP」に有意な差がない
H(1): 「LBありQUIC」のほうが「LBありTCP」よりも有意に大きい

Levene's test for connect_duration:
  Statistic: 46.45760132236707
  P-value: 9.364880224646898e-12
connect_duration: Statistic: 12.717005984049031 P-value: 2.3907798125364235e-37
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のconnect_durationの方が「LBありTCP」より大きい

Levene's test for request_sent_duration:
  Statistic: nan
  P-value: nan
request_sent_duration: Statistic: nan P-value: nan
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

Levene's test for response_first_byte_duration:
  Statistic: 12048.281260317688
  P-value: 0.0
response_first_byte_duration: Statistic: 437.5504977753638 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のresponse_first_byte_durationの方が「LBありTCP」より大きい

Levene's test for full_duration:
  Statistic: 11353.023913496141
  P-value: 0.0
full_duration: Statistic: 403.32081931065323 P-value: 0.0
  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なり統計上は「LBありQUIC」のfull_durationの方が「LBありTCP」より大きい

Levene's test for request_per_second:
  Statistic: 926.6697555672328
  P-value: 1.7415367895391734e-203
request_per_second: Statistic: -10042.88815469913 P-value: 1.0
  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない

```
