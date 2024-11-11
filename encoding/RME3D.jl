using Base.Threads  # Import threading utilities
using Nemo, Primes

### Not scalable because it needs to build a matrix of size n * n where n is the size of the database.

function lexicographic_monomials(m, d)
    monomials = []
    
    # Helper function to recursively build exponent tuples
    function generate_exponents(current, total_degree, max_degree)
        # Base case: If we have m variables, add the current tuple to monomials
        if length(current) == m
            if total_degree <= max_degree
                push!(monomials, current)
            end
            return
        end
        
        # Recursive case: Add each possible exponent for the current variable
        for exponent in 0:max_degree - total_degree
            generate_exponents([current; exponent], total_degree + exponent, max_degree)
        end
    end
    
    # Start recursion with an empty tuple
    generate_exponents([], 0, d)
    return monomials
end

function lexicographic_monomials(d)
    monomials = []
    
    for i in 0:d
        for j in 0:d-i
            for k in 0:d-i-j
                push!(monomials, [i,j,k])
            end
        end
    end
    return monomials
end

function EvaluateMonomials(x, y ,z, cur, d)
    v = Vector{typeof(cur)}(undef, 0)
    push!(v, cur) # Evaluation for (0,0,0) monomial
    cur_i = cur
    for i in 0:d-1
        cur_j = cur_i
        for j in 0:d-i-1
            cur_k = cur_j
            for k in 0:d-i-j-1
                cur_k *= z  # Move from (i, j, k) to (i, j, k+1) by multiplying with z
                push!(v, cur_k)
            end
            cur_j *= y  # Move from (i, j, d-i-j) to (i, j+1, 0) by multiplying with y
            push!(v, cur_j)
        end
        cur_i *= x  # Move from (i, d-i, 0) to (i+1, 0, 0) by multiplying with x
        push!(v, cur_i)
    end
    return v
end

function EvaluateCanonicalSet(monomials, d, q)
    # monomials = lexicographic_monomials(m, d)
    row = 1
    l = length(monomials)

    F = GF(q)
    M = matrix_space(F, l, l)
    A = M()

    @time for mon in monomials
        x, y, z = mon[1], mon[2], mon[3]
        cur = F(1)
        A[row, :] = EvaluateMonomials(x, y, z, cur, d) 
        row += 1
    end
    return A
end

function ParallelEvaluateCanonicalSet(monomials, d, q)
    # monomials = lexicographic_monomials(m, d)
    row = 1
    l = length(monomials)

    F = GF(q)
    M = matrix_space(F, l, l)
    A = M()
    Threads.@threads for row in 1:l
        x, y, z = monomials[row][1], monomials[row][2], monomials[row][3]
        cur = F(1)
        A[row, :] = EvaluateMonomials(x, y, z, cur, d) 
        row += 1
    end

    return A
end

function RME3D(d, message, q)
    monomials = lexicographic_monomials(d)
    F = GF(q)
    A = ParallelEvaluateCanonicalSet(monomials, d, q)
    B = inv(A)
    mess = [F(i) for i in message]
    coeff = mess * B

    codeword = [F(0) for _ in 1:q, _ in 1:q, _ in 1:q]

    for i in 1:lastindex(message)
        codeword[(monomials[i] .+ 1)...] = mess[i]
    end

    count = 0

    for i in 1:q
        for j in 1:q
            for k in 1:q
                if (i+j+k > d + 3)
                    count += 1
                    evl = EvaluateMonomials(i-1, j-1, k-1, F(1), d)
                    codeword[i,j,k] = sum(evl .* coeff)
                end
            end
        end
    end
    @assert lastindex(monomials) + count == q^m
    return codeword
end

