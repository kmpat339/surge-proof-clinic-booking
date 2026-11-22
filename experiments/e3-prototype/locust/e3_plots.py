import json
from pathlib import Path
import pandas as pd
import matplotlib.pyplot as plt

RESULTS = Path("results")

# these are the 8 runs
TTLS = [("0", 0), ("1s", 1), ("10s", 10), ("60s", 60)]
BOOKINGS = ["100ms", "1s"]

TTL_ORDER = [0, 1, 10, 60]
TTL_LABELS = ["no cache", "1s", "10s", "60s"]

# fixed colors 
SERIES = {
    "100ms": ("rush (booking every 100ms)", "orange"),
    "1s": ("normal (booking every 1s)", "cornflowerblue")
}

def load_runs():
    rows = []
    for book in BOOKINGS:
        for ttl_label, ttl_seconds in TTLS:
            prefix = f"e3_ttl{ttl_label}_book{book}"

            # counters from metrics
            with open(RESULTS / f"{prefix}_metrics.json") as f:
                metrics = json.load(f)

            # grab the locust latency stats
            stats = pd.read_csv(RESULTS / f"{prefix}_stats.csv")
            # grab the aggregated row
            agg = stats[stats["Name"] == "Aggregated"].iloc[0]

            rows.append({
                "booking": book,
                "ttl_s": ttl_seconds,
                "store_reads": metrics["store_reads"],
                "hit_rate": metrics["hit_rate"],
                "stale_slot_fraction": metrics["stale_slot_fraction"],
                "median_ms": agg["50%"],
                "p95_ms": agg["95%"],
                "p99_ms": agg["99%"],
            })

    return pd.DataFrame(rows)


def plot_tradeoff(df):
    fig, (ax_load, ax_stale) = plt.subplots(1, 2, figsize = (12, 8))

    for book, (label, color) in SERIES.items():
        run = df[df["booking"] == book].set_index("ttl_s").loc[TTL_ORDER]
        x = range(len(TTL_ORDER))

        ax_load.plot(x, 
                     run["store_reads"], 
                     marker="o", 
                     linewidth=2, 
                     color=color, 
                     label=label
                     )
        ax_stale.plot(
            x, 
            run["stale_slot_fraction"] * 100,
            marker="o",
            linewidth=2,
            color=color,
            label=label
        )
        ax_load.set_yscale("log")
        ax_load.set_ylabel("store reads per 60s run on log scale")
        ax_load.set_title("database load")

        ax_stale.set_ylabel("slots shown that were already booked (%)")
        ax_stale.set_title("staleness")

        for ax in (ax_load, ax_stale):
            ax.grid(axis="y", color="whitesmoke", linewidth=0.8)
            ax.spines[["top", "right"]].set_visible(False)
            ax.set_xticks(range(len(TTL_ORDER)), TTL_LABELS)
            ax.set_label("cache TTL")
        ax_stale.legend(frameon=False)

        fig.suptitle("e3: a short TTL removes most database load. longer TTL adds staleness")
        fig.tight_layout()
        fig.savefig(RESULTS / "e3_fig1_tradeoff.png")


def plot_latency(df):
    fig, (ax_med, ax_p99) = plt.subplots(1, 2, figsize=(12, 6), sharey=True)

    for book, (label, color) in SERIES.items():
        run = df[df["booking"] == book].set_index("ttl_s").loc[TTL_ORDER]
        x = range(len(TTL_ORDER))

        ax_med.plot(x, 
                    run["median_ms"],
                    marker="o",
                    linewidth=2, 
                    color=color,
                    label=label
                    )
        ax_p99.plot(x, 
                    run["p99_ms"],
                    marker="o",
                    linewidth=2,
                    color=color,
                    label=label
                    )
        ax_med.set_ylabel("response time (ms)")
        ax_med.set_title("median request")
        ax_p99.set_title("slowest 1% of requests (p99)")

        for ax in (ax_med, ax_p99):
            ax.grid(axis="y", color="whitesmoke", linewidth=0.8)
            ax.spines[["top", "right"]].set_visible(False)
            ax.set_xticks(range(len(TTL_ORDER)), TTL_LABELS)
            ax.set_label("cache TTL")
            ax.set_ylim(0, 45)
        ax_p99.legend(frameon=False)

        fig.suptitle("e3: caching makes a request faster. the slowest requests still pay for a miss")
        fig.tight_layout()
        fig.savefig(RESULTS / "e3_fig2_latency.png")
    


    

if __name__ == "__main__":
    df = load_runs()
    print(df)
    plot_tradeoff(df)
    plot_latency(df)

