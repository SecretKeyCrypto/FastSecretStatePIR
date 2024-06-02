include("../encoding/RMEncoding.jl")
using CSV, DataFrames, Combinatorics

function find_smallest_d(n, m)
    d = floor(Int, n^(1/m))

    # Check the binomial coefficient and increment d until the condition is satisfied
    while true
        if binomial(m + d, d) >= n
            return d
        end
        d += 1
    end
end

for line in eachline(stdin)
    input, output, arg_q, arg_k, arg_m = split(line, " ")
    q = parse(Int, arg_q)
    k = parse(Int, arg_k)
    m = parse(Int, arg_m)

    df = CSV.read(input, DataFrame, header=false)

    message = collect(Tuple(df[1, :]))

    n = length(message)

    d = find_smallest_d(n, m)
    if (d * k + 1 >= q)
        throw(ErrorException("The length of n ($n) message can't be encoded with field size ($q) and degree ($k)."))
    elseif (n < binomial(m+d, d))
        append!(message, rand(0:q-1, binomial(m+d, d)-n))
    end

    rmc = RME(message, d, q, m)
    df = DataFrame(rmc, :auto)
    CSV.write(output, df, header=false)
    println()
end