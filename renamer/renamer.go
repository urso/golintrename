package renamer

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/tools/refactor/satisfy"
)

// renamer extracted from gorename

type Renamer struct {
	iprog              *loader.Program
	objsToUpdate       map[types.Object]bool
	hadConflicts       bool
	to                 string
	satisfyConstraints map[satisfy.Constraint]bool
	packages           map[*types.Package]*loader.PackageInfo // subset of iprog.AllPackages to inspect
	msets              typeutil.MethodSetCache
	changeMethods      bool
}

var ReportError = func(posn token.Position, message string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", posn, message)
}

func New(prog *loader.Program, to string) *Renamer {
	return &Renamer{
		iprog:        prog,
		objsToUpdate: map[types.Object]bool{},
		to:           to,
		packages:     map[*types.Package]*loader.PackageInfo{},
	}
}

func (r *Renamer) AddPackages(pkgs map[string]*loader.PackageInfo) {
	for _, info := range pkgs {
		r.AddPackage(info)
	}
}

func (r *Renamer) AddAllPackages(pkgs ...*loader.PackageInfo) {
	for _, info := range pkgs {
		r.AddPackage(info)
	}
}

func (r *Renamer) AddPackage(info *loader.PackageInfo) {
	r.packages[info.Pkg] = info
}

// update checks and updates the input program returning the set of updated files.
func (r *Renamer) Update(objs ...types.Object) (map[*token.File]bool, error) {
	for _, obj := range objs {
		if obj, ok := obj.(*types.Func); ok {
			recv := obj.Type().(*types.Signature).Recv()
			if recv != nil && isInterface(recv.Type().Underlying()) {
				r.changeMethods = true
				break
			}
		}
	}

	for _, obj := range objs {
		r.check(obj)
	}
	if r.hadConflicts {
		return nil, errors.New("Conflicts detected")
	}

	return r.doUpdate(), nil
}

func (r *Renamer) doUpdate() map[*token.File]bool {
	// We use token.File, not filename, since a file may appear to
	// belong to multiple packages and be parsed more than once.
	// token.File captures this distinction; filename does not.
	var nidents int
	var filesToUpdate = make(map[*token.File]bool)
	for _, info := range r.packages {
		// Mutate the ASTs and note the filenames.
		for id, obj := range info.Defs {
			if r.objsToUpdate[obj] {
				nidents++
				id.Name = r.to
				filesToUpdate[r.iprog.Fset.File(id.Pos())] = true
			}
		}
		for id, obj := range info.Uses {
			if r.objsToUpdate[obj] {
				nidents++
				id.Name = r.to
				filesToUpdate[r.iprog.Fset.File(id.Pos())] = true
			}
		}
	}

	return filesToUpdate
}
