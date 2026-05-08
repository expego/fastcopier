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

## Proposed Architecture

```
fastcopier/
├── copier.go           # Public API
├── codegen/            # Code generation tools
│   ├── generator.go    # AST-based code generator
│   └── templates.go    # Copy function templates
├── reflect/            # Reflection-based fallback
│   ├── cache.go        # Field mapping cache
│   └── copy.go         # Optimized reflection copier
├── unsafe/             # Unsafe optimizations (optional)
│   └── direct.go       # Direct memory copy for compatible types
└── bench/              # Benchmarks vs competitors
    ├── simple_test.go
    ├── nested_test.go
    └── comparison_test.go
```

## Performance Targets

| Scenario | Target Performance | vs jinzhu/copier | vs tiendc/go-deepcopy |
|----------|-------------------|------------------|----------------------|
| Simple struct (5 fields) | < 50 ns/op | 20-100x faster | 2-5x faster |
| Nested struct (3 levels) | < 200 ns/op | 10-50x faster | 1.5-3x faster |
| Slice of structs (100 items) | < 5000 ns/op | 5-20x faster | 1.2-2x faster |
| With type conversion | < 100 ns/op | 10-30x faster | 1.5-3x faster |

## Key Features to Implement

1. **Zero-config usage**: Works out of the box without code generation
2. **Optional codegen**: Generate optimized code for critical paths
3. **Type conversion**: Automatic conversion between compatible types
4. **Deep copy**: Handle nested structs, slices, maps
5. **Field mapping**: Support struct tags for custom field names
6. **Concurrent-safe**: Thread-safe caching and operations
7. **Generics support**: Type-safe API using Go 1.18+ generics

## Implementation Phases

### Phase 1: Core Reflection Engine (MVP)
- Basic struct-to-struct copy using optimized reflection
- Field mapping cache
- Simple type conversion
- Benchmark framework

### Phase 2: Code Generation
- AST-based code generator
- Template-based copy function generation
- Integration with go:generate

### Phase 3: Advanced Features
- Deep copy for complex types
- Custom field mapping via tags
- Unsafe optimizations (opt-in)
- Partial copy (field masking)

### Phase 4: Optimization & Polish
- Profile-guided optimization
- Memory allocation reduction
- Documentation and examples
- Production readiness

## Next Steps

1. Set up project structure
2. Implement Phase 1 (reflection-based MVP)
3. Create benchmark suite comparing all 4 libraries
4. Iterate on optimizations based on benchmark results
5. Add code generation in Phase 2
