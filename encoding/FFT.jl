using Nemo

# a is a vector of coefficients
# ω is d-th root of unity
function fft!(F, a, ω, d)
    # Pad the array if its length is less than the desired root of unity order d
    n = length(a)
    if n < d
        for _ in 1:(d - n)
            push!(a, F(0))  # Pad with zeros to length d
        end
    elseif n > d
        throw(DomainError("Input length cannot be greater than the root of unity order d"))
    end

    # Base case: if the input length is 1, just return it
    n = length(a)
    if n == 1
        return a
    elseif n % 2 != 0
        throw(DomainError("Input length must be a power of 2"))
    end

    # Recursive FFT: Separate even and odd terms
    even_terms = fft!(F, a[1:2:end], ω^2, d ÷ 2)
    odd_terms = fft!(F, a[2:2:end], ω^2, d ÷ 2)

    # Combine results using the root of unity
    ω_power = F(1)  # Start with ω^0
    for i in 1:n÷2
        t = ω_power * odd_terms[i]
        a[i] = even_terms[i] + t
        a[i + n÷2] = even_terms[i] - t
        ω_power *= ω  # Progress ω_power correctly for each pair
    end

    return a
end


# Define the Inverse FFT (IFFT) function
function ifft!(F, a, ω_inv, d)
    n = length(a)    
    fft!(F, a, ω_inv, d)
    # Scale each term by 1/n to complete the inverse transformation
    d_inv = F(1) / F(d)
    for i in 1:n
        a[i] *= d_inv
    end

    return a
end

function prime_factors(n)
    factors = Dict{Int, Int}()
    d = 2
    while d * d <= n
        while n % d == 0
            factors[d] = get(factors, d, 0) + 1
            n ÷= d
        end
        d += 1
    end
    if n > 1
        factors[n] = 1
    end
    return factors
end

# Function to check if g is a primitive root
function is_primitive_root(F, g, q)
    # Find the factors of q - 1
    factors = prime_factors(q - 1)
    # Check if g^( (q-1)/p ) != 1 for each prime factor p of (q-1)
    for (p, _) in factors
        if g^((q - 1) ÷ p) == F(1)
            return false
        end
    end
    return true
end

# Find a primitive root by testing random elements
function find_primitive_root(F, q)
    for g in F
        if g != F(0) && is_primitive_root(F, g, q)
            return g
        end
    end
    error("No primitive root found, check field parameters")
end

function is_power_of_2(x)
    return x > 0 && (x & (x - 1)) == 0
end