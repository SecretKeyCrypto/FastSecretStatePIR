using Nemo
using Base.Threads

include("RMEncoding.jl")

function LargeRecordRME2D(message, q, d)
    n = length(message)
    max_records = findmax(message)[1]
    slice = length(digits(max_records, base = q))
    matrix = zeros(Int16, slice, n)

    @threads for i in 1:n
        vec = digits(message[i], base=q)
        matrix[1:length(vec),i] = vec
    end

    results = Array{Matrix{Int}, 1}(undef, slice)

    @threads for i in 1:slice
        results[i] = RME2D(matrix[i, :], q, d)
    end

    return results
end

function LargeRecordRME2DInputMatrix(messageMatrix, q, d)
    (slice, _) = size(messageMatrix)

    results = Array{Matrix{Int}, 1}(undef, slice)

    @threads for i in 1:slice
        results[i] = RME2D(messageMatrix[i, :], q, d)
    end

    return results
end