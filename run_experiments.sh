#!/bin/sh
# Usage: ./run_experiments.sh [reps]
# Builds the program, runs seq/static/steal for N in {2,4,6,8,12} on every
# preset, writes results.csv (median of reps) and speedup_*.png.
set -e
cd "$(dirname "$0")"
REPS=${1:-3}
go build -o proj3 .
echo "preset,mode,threads,time_ms" > results.csv
median() { sort -n | awk '{a[NR]=$1} END{print a[int((NR+1)/2)]}'; }
timeit() { # preset mode threads
  for _ in $(seq "$REPS"); do
    ./proj3 -preset "$1" -mode "$2" -t "$3" | sed -n 's/^time_ms=//p'
  done | median
}
for preset in full seahorse uniform; do
  echo "$preset,seq,1,$(timeit $preset seq 1)" >> results.csv
  for mode in static steal; do
    for t in 2 4 6 8 12; do
      echo "$preset,$mode,$t,$(timeit $preset $mode $t)" >> results.csv
    done
  done
done
python3 plot.py results.csv
cat results.csv
