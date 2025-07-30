import numpy as np
import pandas as pd
from scipy import stats
import matplotlib.pyplot as plt
import seaborn as sns
import glob
import json

# 直列テストシナリオ
# 負荷が少ないため全体的に処理速度が速くまた有意な差も以下条件では確認できなかった
#LB = "metrics/20250730063930/lb_exist_*.json"
#NO_LB = "metrics/20250730065416/lb_not_exist_*.json"

# 並列テストシナリオ
# 1000000回を並列数無制限(可能な限り)で実行
# こちらの場合テスト環境のノイズの影響が強すぎたと思われ有意な差が計算上は出たが
# あんまり意味のあるデータではないように思われる(以下2パターンで真逆の結果が出るなどしていた)
#LB= "metrics/20250730082526/lb_exist_*.json"
#NO_LB = "metrics/20250730082526/lb_not_exist_*.json"
#LB="metrics/20250730084603/lb_exist_*.json"
#NO_LB="metrics/20250730084603/lb_not_exist_*.json"

# 100000回を並列回数1000に制限して実行
# 直列実行シナリオと同じく有意な差はないがやはり環境要因が大きなファクターを占めているため
# 高負荷環境試験は別途ちゃんと調整して行う方が良いであろう(本番環境もってかないとわかんない....)
LB="metrics/20250730122515/lb_exist_*.json"
NO_LB="metrics/20250730122515/lb_not_exist_*.json"

def load_metrics(file_pattern):
    """
format is:
type Metrics struct {
	ID                        int           `json:"id"`
	StartTime                 time.Time     `json:"start_time,omitempty"`
	DNSDuration               time.Duration `json:"dns_duration,omitempty"`
	ConnectDuration           time.Duration `json:"connect_duration,omitempty"`
	RequestSentDuration       time.Duration `json:"request_sent_duration,omitempty"`
	ResponseFirstByteDuration time.Duration `json:"response_first_byte_duration,omitempty"`
	FullDuration              time.Duration `json:"full_duration,omitempty"`
	ResponseBody              string        `json:"response_body,omitempty"`
}

type metricsOutput struct {
	Data      []*Metrics `json:"data"`
	Summary   Metrics    `json:"summary"`
	Variance  Metrics    `json:"variance"`
	StartTime time.Time  `json:"start_time"`
	EndTime   time.Time  `json:"end_time"`
}
"""
    files = glob.glob(file_pattern)
    metrics_list = []
    for file in files:
        print(f"Loading metrics from {file}")
        with open(file, 'r') as f:
            data = json.load(f)
            print(f"Loaded {len(data['data'])} metrics entries.")
            # Convert to DataFrame
            df = pd.DataFrame(data['data'])
            metrics_list.append(df)
            print(f"DataFrame shape: {df.shape}")
    return pd.concat(metrics_list, ignore_index=True)

lbdf = load_metrics(LB)
noldf = load_metrics(NO_LB)

def calculate_statistics(df):
    """
    Calculate statistics for the given DataFrame.
    Returns a dictionary with mean, median, and variance for each metric.
    """
    stats_dict = {}
    metrics = [ 'connect_duration', 'request_sent_duration', 
               'response_first_byte_duration', 'full_duration']
    
    for metric in metrics:
        stats_dict[metric] = {
            "unit": "nanoseconds",
            "data_count": len(df[metric]),
            'sum': float(df[metric].sum()),
            'mean': float(df[metric].mean()),
            'median': float(df[metric].median()),
            'variance': float(df[metric].var()),
            'std_dev': float(df[metric].std()),
            "quantiles": df[metric].quantile([0.25, 0.5, 0.75]).to_dict(),
            'skewness': float(df[metric].skew()),
            'kurtosis': float(df[metric].kurtosis()),
            'min': float(df[metric].min()),
            'max': float(df[metric].max()),
        }
    return stats_dict


stats_lb = calculate_statistics(lbdf)
stats_no_lb = calculate_statistics(noldf)
print("Load Balancer Exists Metrics Statistics:")
print(json.dumps(stats_lb, indent=2))
print("Load Balancer Does Not Exist Metrics Statistics:")
print(json.dumps(stats_no_lb, indent=2))
print("H(0): LBが存在する場合でも、各メトリックの平均値はない場合と変わらない")
print("H(1): LBが存在する場合、各メトリックの平均値はない場合に比べて有意に大きくなる。")
alpha = 0.05 # 有意水準
# Levene's test
for metric in stats_lb.keys():
    print(f"\nLevene's test for {metric}:")
    levene_stat, levene_p_value = stats.levene(lbdf[metric], noldf[metric])
    print(f"  Statistic: {levene_stat:.4f}")
    print(f"  P-value: {levene_p_value:.4f}")
    #if levene_p_value < alpha:
    #    print("  => P-valueが有意水準より小さいため、分散は等しくないと判断されます。")
    #    use_equal_var = False
    #else:
    #    print("  => P-valueが有意水準以上であるため、分散が等しいか等しくないかどうか証拠が足りない")
    #    use_equal_var = True
    use_equal_var = False
    t_stat, p_value = stats.ttest_ind(lbdf[metric], noldf[metric], equal_var=use_equal_var,alternative='greater')
    print(f"{metric}: p-value = {p_value}")

    if p_value < alpha:
        print(f"  => P-valueが有意水準より小さいため、2つのグループの平均は統計的に有意に異なりLBが存在する場合オーバヘッドがあるといえる")
    else:
        print(f"  => P-valueが有意水準以上であるため、2つのグループの平均は統計的に有意に異なるかどうか証拠が足りない")

