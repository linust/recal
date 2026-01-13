# Task 4: Normalize Lodge Names (Handle Possessive Variants)

## Status
✅ **COMPLETE** - Fully implemented

## Implementation Details

The normalization is implemented through a multi-layer approach:

1. **Configuration** ([config.yaml](../../config.yaml))
   - Lodge names stored in canonical form (without possessive 's')
   - Example: "Sundsvall" not "Sundsvalls"

2. **Matching Logic** ([internal/filter/filter.go:135-143](../../internal/filter/filter.go#L135-L143))
   - Automatically makes trailing 's' optional in regex pattern
   - `"Sundsvall"` becomes `"Sundsvalls?"` in the pattern
   - Matches both "Sundsvall PB:" and "Sundsvalls PB:"
   - Avoids false matches: "Borås" stays as "Borås" (not "Boråss?")

3. **UI Display** ([internal/server/server.go:895-916](../../internal/server/server.go#L895-L916))
   - `GetLodges()` returns canonical names from config
   - Swedish collation sorting (å, ä, ö after z)
   - UI shows single entry "Sundsvall"

## Overview

Improve lodge filtering to handle possessive variations in lodge names.

## Goals

1. **Normalize Lodge Display**: Show single canonical name in UI (without possessive 's')
2. **Smart Matching**: Single selection matches all variants (with/without 's')
3. **Maintain Accuracy**: Don't over-normalize (e.g., don't match unrelated lodges)

## Requirements

**FR1: Lodge Name Normalization**
- [ ] Extract lodge names from upstream feed
- [ ] Normalize possessive variants to base form
- [ ] Rule: If lodge name ends with 's', create base form without 's'
- [ ] Store both canonical form and variants for matching

**FR2: UI Display**
- [ ] Show only canonical (base) form in lodge checkbox list
- [ ] Example: Show "Sundsvall" not both "Sundsvall" and "Sundsvalls"
- [ ] Maintain Swedish alphabetical sorting

**FR3: Filter Matching**
- [ ] When user selects "Sundsvall", match both variants:
  - "Sundsvall PB:"
  - "Sundsvalls PB:"
- [ ] Apply to all lodges, not just specific examples
- [ ] Case-insensitive matching for robustness

**FR4: Backward Compatibility**
- [ ] Existing filter URLs continue to work
- [ ] If URL contains `Loge=Sundsvalls`, normalize to base form
- [ ] No breaking changes to API

## Technical Design

### Normalization Algorithm

**Step 1: Extract lodge variants from upstream**
```go
// Parse event: "Grad 4, Sundsvalls PB: Meeting"
// Extract: "Sundsvalls"
```

**Step 2: Normalize to canonical form**
```go
func normalizeLodgeName(name string) string {
    // Trim whitespace
    name = strings.TrimSpace(name)

    // If ends with 's', remove it for canonical form
    // But only if it's likely a possessive (length > 3 to avoid "As", "Os", etc.)
    if len(name) > 3 && strings.HasSuffix(strings.ToLower(name), "s") {
        return name[:len(name)-1]
    }

    return name
}

// Examples:
// "Sundsvalls" → "Sundsvall"
// "Göta" → "Göta" (no change)
// "Zions" → "Zion"
// "As" → "As" (too short, no change)
```

**Step 3: Build variant map**
```go
type LodgeVariants struct {
    Canonical string   // "Sundsvall"
    Variants  []string // ["Sundsvall", "Sundsvalls"]
}

// Build from upstream
variants := map[string]*LodgeVariants{
    "Sundsvall": {
        Canonical: "Sundsvall",
        Variants: ["Sundsvall", "Sundsvalls"],
    },
    "Göta": {
        Canonical: "Göta",
        Variants: ["Göta"],
    },
}
```

**Step 4: Return canonical list to UI**
```json
{
  "lodges": ["Göta", "Sundsvall", "Zion"]
}
```

**Step 5: Match against all variants**
```go
func matchesLodge(eventSummary string, selectedLodge string) bool {
    // Normalize user selection
    canonical := normalizeLodgeName(selectedLodge)

    // Get all variants for this canonical form
    variants := getLodgeVariants(canonical)

    // Check if event matches any variant
    for _, variant := range variants {
        pattern := variant + " PB:"
        if strings.Contains(eventSummary, pattern) {
            return true
        }
    }

    return false
}
```

### Implementation Points

**Files to Modify:**

1. **`internal/server/server.go`** - `GetLodges()` function
   - Extract lodge names and variants
   - Normalize to canonical forms
   - Return deduplicated list

2. **`internal/filter/loge.go`** (or wherever loge filter lives)
   - Update matching logic to check all variants
   - Normalize filter parameter before matching

3. **Tests**
   - Unit tests for normalization logic
   - Test matching with possessive variants
   - Test edge cases (short names, special characters)

### Edge Cases to Handle

**EC1: Short names**
```
"As" → "As" (don't remove 's', too short)
"Os" → "Os" (don't remove 's', too short)
```

**EC2: Non-possessive 's'**
```
"Norrlands" → "Norrland" (okay, possessive)
"Anders" → "Ander" (might be wrong, but acceptable)
```

**EC3: Multiple consecutive 's'**
```
"Sundsvalls" → "Sundsvall" (remove one 's')
```

**EC4: Case variations**
```
"SUNDSVALLS PB:" → matches "Sundsvall"
"sundsvalls pb:" → matches "Sundsvall"
```

### Alternative Approaches Considered

**Option A: Fuzzy matching (rejected)**
- Too complex, may match unintended lodges
- Hard to explain to users

**Option B: Allow both in UI with grouping (rejected)**
- Still confusing
- Doesn't solve the selection problem

**Option C: Manual mapping (rejected)**
- Requires maintenance
- Brittle when new lodges added

**Option D: Suffix removal (selected)**
- Simple, predictable rule
- Automatic, no maintenance
- Works for Swedish possessive pattern

## Testing Requirements

**TC1: Normalization**
- [ ] "Sundsvalls" normalizes to "Sundsvall"
- [ ] "Göta" remains "Göta"
- [ ] "Zions" normalizes to "Zion"
- [ ] "As" remains "As" (too short)

**TC2: Variant Detection**
- [ ] Upstream with "Sundsvall PB:" and "Sundsvalls PB:" creates single entry
- [ ] Both variants stored in variant map

**TC3: Filter Matching**
- [ ] Filter `Loge=Sundsvall` matches "Sundsvall PB:" events
- [ ] Filter `Loge=Sundsvall` matches "Sundsvalls PB:" events
- [ ] Filter `Loge=Göta` only matches "Göta PB:" (no variants)

**TC4: UI Display**
- [ ] `/api/lodges` returns deduplicated canonical names
- [ ] No duplicate/variant entries in response
- [ ] Proper Swedish sorting maintained

**TC5: Backward Compatibility**
- [ ] Old URL `?Loge=Sundsvalls` still works (normalizes internally)
- [ ] Filter behavior unchanged for lodges without variants

## Benefits

1. **Better UX**: Users see clean, deduplicated lodge list
2. **No Missed Events**: Single selection catches all variants
3. **Automatic**: Works for all lodges, not just known cases
4. **Simple Rule**: Easy to understand and maintain
5. **No Breaking Changes**: Existing URLs continue to work

## Potential Issues

**Issue 1: Over-normalization**
- Might normalize names that shouldn't be normalized
- Mitigation: Minimum length requirement (> 3 chars)

**Issue 2: Legitimate different lodges**
- What if "Sundsvall" and "Sundsvalls" are different lodges?
- Mitigation: Unlikely in Swedish Freemasonry context; can be overridden with manual mapping if needed

**Issue 3: Non-Swedish lodges**
- Rule is Swedish-specific
- Mitigation: Still works, just might not normalize correctly for other languages (acceptable)

## Estimated Effort

- **Design/Planning:** 1 hour
- **Implementation:** 2-3 hours
- **Testing:** 1-2 hours
- **Documentation:** 30 minutes
- **Total:** ~5 hours

## Priority

**Recommended:** After Task 3, before or alongside Task 1

**Rationale:**
- Improves core filtering functionality
- Small, focused change
- High user value (reduces confusion)
- No dependencies on other tasks
