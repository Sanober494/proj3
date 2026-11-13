# proj3: parallel Mandelbrot

    cd proj3
    go build -o proj3 .
    ./proj3 -mode steal -t 8 -preset full -o out.png -stats
    ./proj3 -h                    # usage
    ./run_experiments.sh          # full experiment set -> results.csv + speedup_*.png
    go test -race ./deque         # deque correctness


