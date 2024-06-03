using Combinatorics
using Printf, Primes


function unionbound(k,l)
    su = 0
    for s in 0:k+l
        su += singleTerm(k,l,s)
    end
    println(su)
end



function singleTerm(k,l, s)
    return binomial(k+l, s)* (-1)^s * binomial(binomial(k+l-s, l), k+1)
end

function cal_alpha(ℓ, j)
    α_0 = 1
    α_1 = binomial(ℓ+1, ℓ)
    
    Α = []
    push!(Α, α_0)
    push!(Α, α_1)

    for i in 2:j
        α = 0
        sign = 1
        for k in i-1:-1:0
            α += sign * binomial(ℓ + i, ℓ + k) * Α[k+1] 
            # println(sign, "||", ℓ + i, "||",  ℓ + k, "||",  Α[k+1])
            sign *= -1
        end
        push!(Α, α)
    end

    for i in 1:lastindex(Α)
        Α[i] *= (-1)^(i-1)
    end
    println("α: ", Α)
end

cal_alpha(5,10)

C = []
for i in 0:10
    push!(C, factorial(5+i)*exp(i))
end
println(C)

function count_down(A, l, total)
    B = []
    q = l + length(A)
    push!(B, A[1])

    for i in 2:lastindex(A)
        α = A[i]
        for j in 1:lastindex(B)
            println(q+1-j,"||", q+1-i, "||", B[j])
            α -= binomial(q+1-j, q+1-i) * B[j]
        end
        push!(B, α)
    end

    for i in 1:lastindex(B)
        total -= binomial(q+1-i, l) * B[i]
    end

    println(B, total)

end

# count_down([1,7,22, 38], 2, 38)



#################################################################################
#     
# Find the best q and n given m and k
function find_best_q_and_n(m::Int, k::Int)
    target = 2^33
    d = 100
    while true
        n = binomial(m + d, m)
        q = nextprime(d * k)
        if q > 1  # log2(q) is undefined for q <= 1
            value = n * log2(q)
            if value >= target
                return q, log2(q), log2(n), d
            end
        end
        d += 1
    end
end

# Example usage
m = 4
k = 5
for k in range(2,5)
    q, logq, n, d = find_best_q_and_n(m, k)
    # println(q, " ", logq, " ", n, " ", d)
    @printf("Best q: %d, Best n: %d, Best d: %d\n", q, n, d)
end
