# Research: High-Performance Go Struct Copy Library

## Target Libraries Analysis

### 1. github.com/jinzhu/copier
**Approach**: Runtime reflection
**Key Characteristics**:
- Uses `reflect` package for field mapping
- Supports deep copy, field name matching, type conversion
- Allocates heavily due to reflection overhead
- Performance bottleneck: repeated reflection calls per field

**Typical Performance**: ~1000-5000 ns/op for simple structs

### 2. github.com/ulule/deepcopier
**Approach**: Runtime reflection with caching
**Key Characteristics**:
- Caches field mappings to reduce reflection overhead
- Supports field tags for custom mapping
- Still uses reflection for actual copying
- Better than jinzhu/copier but still reflection-bound

**Typical Performance**: ~800-3000 ns/op for simple structs

### 3. github.com/tiendc/go-deepcopy
**Approach**: Code generation
**Key Characteristics**:
- Generates type-specific copy functions at compile time
- Zero reflection overhead at runtime
- Requires code generation step
- Best performance among existing solutions

**Typical Performance**: ~100-500 ns/op for simple structs

### 4. github.com/mennanov/fieldmask-utils
**Approach**: Reflection with field masking
**Key Characteristics**:
- Focuses on partial updates (field masking)
- Uses reflection for dynamic field selection
- Optimized for API use cases (protobuf field masks)
- Not optimized for full struct copying

**Typical Performance**: ~2000-8000 ns/op (depends on mask complexity)

## Performance Bottlenecks in Existing Libraries

1. **Reflection Overhead**: Most libraries use `reflect.Value.Set()` which is 10-50x slower than direct assignment
2. **Memory Allocations**: Reflection creates temporary values and interfaces
3. **Type Checking**: Runtime type validation on every copy operation
4. **No Inlining**: Reflection prevents compiler optimizations
5. **Cache Misses**: Dynamic dispatch hurts CPU cache performance

## Optimization Strategies for Our Library

### Strategy 1: Hybrid Approach (Code Generation + Reflection Fallback)
- Generate optimized code for known types at compile time
- Fall back to cached reflection for unknown types
- Best of both worlds: performance + flexibility

### Strategy 2: Zero-Allocation Design
- Pre-allocate field mapping cache
- Reuse reflection metadata
- Avoid interface{} conversions where possible
- Use unsafe pointers for direct memory access (when safe)

### Strategy 3: Compiler-Friendly Patterns
- Use inline-able functions
- Avoid dynamic dispatch
- Leverage Go 1.18+ generics for type-safe operations
- Enable escape analysis optimizations

### Strategy 4: Smart Caching
- Cache field mappings by type pair (src -> dst)
- Use sync.Map for concurrent access
- Lazy initialization of cache entries
- Memory-efficient cache eviction

## Notes

This document captures initial market and performance research context only.
Current architecture, implementation details, and benchmarks live in:

- `ARCHITECTURE.md`
- `docs/BENCHMARKS.md`
- `README.md`
