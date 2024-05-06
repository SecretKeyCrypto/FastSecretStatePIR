using Polynomials, Nemo

################################################################################
#                 Uni-Variate Polynomial Evaluation on all points of Fp
################################################################################
function uniVFastEvaluate(f::Vector{T}, p::Integer) where {T<:Integer}
    return uniVFastEvaluate(Polynomial(f), p)
end

function uniVWithTree(f::FqPolyRingElem, p::Integer, modTree::Vector{FqPolyRingElem})
    result = Vector{FqPolyRingElem}()
    Fq, _ = finite_field(p, 1, "x")
    R, x = polynomial_ring(Fq, "x")
    n = lastindex(modTree)
    # Mod f of (x^p - x) which equals 0 in ring of Fp
    if length(f) > p
        f = rem(f, x^p - x)
    end

    # Recursively modulo the moduloTree, the lowest level would be the corresponding evaluation
    push!(result, rem(f, modTree[n]))
    recModulo(result, modTree, 1, 1, n)

    # May modulo p if necessary, leave it in Z for now
    return circshift(result[n:-1:n-p+1], 1)
end

function uniVFastEvaluate(f::FqPolyRingElem, p::Integer)
    modTree = buildModTree(p)
    result = Vector{FqPolyRingElem}()
    Fq, _ = finite_field(p, 1, "x")
    R, x = polynomial_ring(Fq, "x")
    n = lastindex(modTree)
    # Mod f of (x^p - x) which equals 0 in ring of Fp
    if length(f) > p
        f = rem(f, x^p - x)
    end

    # Recursively modulo the moduloTree, the lowest level would be the corresponding evaluation
    push!(result, rem(f, modTree[n]))
    recModulo(result, modTree, 1, 1, n)

    # May modulo p if necessary, leave it in Z for now
    return circshift(result[n:-1:n-p+1], 1)
end

################################################################################
#                 Uni-Variate Polynomial Interpolation on Fp
################################################################################
function buildLinearCombinationTree(p::Integer, n::Integer, x::Vector{}, y::Vector{})
    Fq, _ = finite_field(p, 1, "x")
    R, _ = polynomial_ring(Fq, "x")
    tree = map(i -> R([Fq(-i),Fq(1)]), collect(1:p))
    m = prod(tree)
    a = divrem.(m, tree)
    mprime = sum(a[2])
    uniVFastEvaluate(mprime, p)
end

################################################################################
#                 Build recursive modulo tree for fast multipoint Evaluation
#                 Ref: Modern Computer Algebra Chapter 10.1
################################################################################
function buildModTree(q::Integer)
    Fq = finite_field(q, 1, "x")[1]
    R = polynomial_ring(Fq, "x")[1]
    tree = map(i -> R([Fq(-i),Fq(1)]), collect(1:q))
    append!(tree, [R([Fq(1)]) for i in 1:(Integer(2^ceil(log2(q))) - q)])

    # Build upper laypers recursively
    recBuildModTree(tree, lastindex(tree))
    return tree
end

function recBuildModTree(tree, n)
    # log2(n) denotes the layer of the tree
    if n == 1
        return
    end

    # Build parent node from lower degree children
    for i in (lastindex(tree) - n + 1): 2: lastindex(tree)
        # TODO: FFT
        push!(tree, tree[i] * tree[i+1])
    end
    
    recBuildModTree(tree, n ÷ 2)
end

# Recursively modulo the Polynomial of the tree Head-to-Bottom
function recModulo(rec, tree, start, en, n)

    for i in start:en
        push!(rec, rem(rec[i], tree[(n-2*i +1)]))
        push!(rec, rem(rec[i], tree[(n-2*i)]))
    end
    
    if en * 2 + 1 < lastindex(tree)
        recModulo(rec, tree, en + 1, en * 2 + 1, n)
    end
end

################################################################################
#                 Computational Utilities
################################################################################
function genZeroPointPoly(i, p)
    # Convert i to an element of Fq and create a polynomial -i + x
    Fq, x = finite_field(p, 1, "x")
    return -Fq(i) + x
end

function toInt(i::FqFieldElem)
    return parse(Int64, string(i))
end

function toInt(i::FqPolyRingElem)
    if length(i) > 1
        throw(ArgumentError("This only works for constant polynomials."))
    end

    return toInt(coeff(i, 0))
end
