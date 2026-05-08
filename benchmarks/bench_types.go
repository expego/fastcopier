package benchmarks

import "fmt"

// ── Simple / Nested / Complex ─────────────────────────────────────────────────
// These three types are the generator's primary targets.
// fastcopier-gen emits CopyBenchSimpleToBenchSimple, CopyBenchNestedToBenchNested
// and CopyBenchComplexToBenchComplex into bench_copy_gen.go.

// BenchSimple is a flat struct: all fields are scalars.
// The generator emits a direct-assignment function (zero allocations).
type BenchSimple struct {
	Name   string
	Age    int
	Email  string
	Score  float64
	Active bool
}

// BenchNested contains a flat struct field plus two scalar slices.
// The generator emits capacity-aware slice copies (builtin copy).
type BenchNested struct {
	ID      int
	Profile BenchSimple
	Tags    []string
	Scores  []int
}

// BenchComplex contains a nested struct, a slice of flat structs, and a flat map.
// The generator emits: recursive call for Nested, builtin copy for Items (flat
// slice), and a range loop for Metadata.
type BenchComplex struct {
	ID       int
	Name     string
	Nested   BenchNested
	Items    []BenchSimple
	Metadata map[string]string
}

// ── Deep (Organisation) ───────────────────────────────────────────────────────
// These types have pointer fields and circular references; the generator does not
// yet handle them.  They remain here so that the deep benchmarks still compile.

type Address struct {
	Street  string
	City    string
	Country string
	Zip     string
}

type ContactInfo struct {
	Email   string
	Phone   string
	Address Address
}

type Role struct {
	ID          int
	Name        string
	Permissions []string
}

type Department struct {
	ID      int
	Name    string
	Budget  float64
	Manager *Employee
}

type Employee struct {
	ID         int
	FirstName  string
	LastName   string
	Age        int
	Salary     float64
	Active     bool
	Contact    ContactInfo
	Roles      []Role
	Tags       map[string]string
	Scores     []float64
	Department *Department
	ReportsTo  *Employee
}

type Organisation struct {
	ID          int
	Name        string
	Founded     int
	Departments []Department
	Employees   []Employee
	Metadata    map[string]string
	HeadOffice  Address
}

// makeOrganisation builds a realistic deep object graph for benchmarking.
func makeOrganisation() Organisation {
	mgr := &Employee{
		ID: 1, FirstName: "Alice", LastName: "Smith", Age: 45, Salary: 120000, Active: true,
		Contact: ContactInfo{
			Email: "alice@corp.com", Phone: "+1-555-0100",
			Address: Address{Street: "1 Main St", City: "Springfield", Country: "US", Zip: "12345"},
		},
		Roles:  []Role{{ID: 1, Name: "Manager", Permissions: []string{"read", "write", "admin"}}},
		Tags:   map[string]string{"level": "senior", "team": "platform"},
		Scores: []float64{9.1, 8.7, 9.5, 9.0},
	}

	dept := &Department{ID: 10, Name: "Engineering", Budget: 5_000_000, Manager: mgr}
	mgr.Department = dept

	employees := make([]Employee, 10)
	for i := range employees {
		employees[i] = Employee{
			ID:        100 + i,
			FirstName: "Employee",
			LastName:  fmt.Sprintf("No%d", i),
			Age:       25 + i,
			Salary:    60000 + float64(i*1000),
			Active:    i%2 == 0,
			Contact: ContactInfo{
				Email: fmt.Sprintf("emp%d@corp.com", i),
				Phone: fmt.Sprintf("+1-555-%04d", i),
				Address: Address{
					Street:  fmt.Sprintf("%d Elm St", i+1),
					City:    "Springfield",
					Country: "US",
					Zip:     fmt.Sprintf("%05d", 10000+i),
				},
			},
			Roles: []Role{
				{ID: i + 1, Name: "Engineer", Permissions: []string{"read", "write"}},
				{ID: i + 100, Name: "Reviewer", Permissions: []string{"read"}},
			},
			Tags:       map[string]string{"level": "mid", "squad": fmt.Sprintf("squad-%d", i%3)},
			Scores:     []float64{7.0 + float64(i)*0.1, 8.0, 7.5},
			Department: dept,
			ReportsTo:  mgr,
		}
	}

	depts := make([]Department, 3)
	for i := range depts {
		depts[i] = Department{
			ID: i + 1, Name: fmt.Sprintf("Dept-%d", i+1),
			Budget: float64((i + 1) * 1_000_000), Manager: mgr,
		}
	}

	return Organisation{
		ID:          1,
		Name:        "Acme Corp",
		Founded:     1990,
		Departments: depts,
		Employees:   employees,
		Metadata:    map[string]string{"region": "NA", "tier": "enterprise", "plan": "pro"},
		HeadOffice:  Address{Street: "100 Corp Ave", City: "Metropolis", Country: "US", Zip: "99999"},
	}
}

// manual deep-copy helpers used by BenchmarkManual_Deep.

func copyAddress(src Address) Address { return src }

func copyContactInfo(src ContactInfo) ContactInfo {
	return ContactInfo{Email: src.Email, Phone: src.Phone, Address: copyAddress(src.Address)}
}

func copyRole(src Role) Role {
	perms := make([]string, len(src.Permissions))
	copy(perms, src.Permissions)
	return Role{ID: src.ID, Name: src.Name, Permissions: perms}
}

func copyEmployee(src *Employee, visited map[*Employee]*Employee) *Employee {
	if src == nil {
		return nil
	}
	if dst, ok := visited[src]; ok {
		return dst
	}
	dst := &Employee{
		ID:        src.ID,
		FirstName: src.FirstName,
		LastName:  src.LastName,
		Age:       src.Age,
		Salary:    src.Salary,
		Active:    src.Active,
		Contact:   copyContactInfo(src.Contact),
	}
	visited[src] = dst
	dst.Roles = make([]Role, len(src.Roles))
	for i, r := range src.Roles {
		dst.Roles[i] = copyRole(r)
	}
	dst.Tags = make(map[string]string, len(src.Tags))
	for k, v := range src.Tags {
		dst.Tags[k] = v
	}
	dst.Scores = make([]float64, len(src.Scores))
	copy(dst.Scores, src.Scores)
	dst.Department = copyDepartment(src.Department, visited)
	dst.ReportsTo = copyEmployee(src.ReportsTo, visited)
	return dst
}

func copyDepartment(src *Department, visited map[*Employee]*Employee) *Department {
	if src == nil {
		return nil
	}
	return &Department{
		ID:      src.ID,
		Name:    src.Name,
		Budget:  src.Budget,
		Manager: copyEmployee(src.Manager, visited),
	}
}
