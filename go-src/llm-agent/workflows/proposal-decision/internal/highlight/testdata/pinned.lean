import Mathlib

/- Outer comment
   /- nested 中文 🐕 <tag> & -/
   still a comment -/
namespace Demo

def greeting : String := "hello \"world\""
def twice (n : Nat) : Nat := n + n

theorem identity (α : Type) (x : α) : x = x := by
  rfl

-- Unicode binders and operators
example (n : Nat) : ∀ x : Nat, x ≤ x := by
  intro x
  exact Nat.le_refl x

end Demo
