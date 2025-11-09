# Customizing ReCal

This document explains how to customize ReCal for your specific use case.

## Overview

**ReCal** (as in Re-Calendar, Re-Calculate or Regexp Calendar Filter) is a **generic** proxy that can filter any iCal feed using various filters and acitons. However, it also supports **custom filter expansions** for domain-specific filtering needs.

## Generic Usage (No Customization Needed)

The application works out of the box with basic regex filtering:

```
# Filter by text in SUMMARY field (default)
/query?pattern=Meeting

# Filter by specific field
/query?field=DESCRIPTION&pattern=urgent

# Multiple filters (AND logic)
/query?field1=SUMMARY&pattern1=Meeting&field2=LOCATION&pattern2=Room%20A

# Use regex patterns
/query?pattern=Project%20[ABC]
```

Configuration: Use `config.yaml.example` as your starting point.

## Custom Filter Expansions

For domain-specific filtering, you can define **custom filter expansions** in your config file. These expand URL parameters into regex patterns.

### Example: Par Bricole (Swedish Freemason Calendar)

The application was originally built for filtering Par Bricole calendar events. See `config-parbricole.yaml.example` for a complete example.

#### Custom Filters Defined:

1. **Grad Filter**: Filter by masonic degree (1-10)
   ```yaml
   grad:
     field: "SUMMARY"
     pattern_template: "Grad %s"
   ```

   Usage: `/query?Grad=4`
   - Keeps: Grad 1, 2, 3, 4
   - Filters out: Grad 5, 6, 7, 8, 9, 10

2. **Loge Filter**: Filter by lodge name with custom patterns
   ```yaml
   loge:
     field: "SUMMARY"
     patterns:
       "Moderlogen":
         template: "PB, %s:"
       default:
         template: "%s PB:"
   ```

   Usage: `/query?Loge=Göta,Borås`
   - Filters out events from "Göta PB:" and "Borås PB:"

3. **ConfirmedOnly**: Keep only confirmed events
   ```yaml
   confirmed_only:
     field: "STATUS"
     pattern: "CONFIRMED"
   ```

   Usage: `/query?ConfirmedOnly=true`

4. **Installt**: Remove cancelled events
   ```yaml
   installt:
     field: "SUMMARY"
     pattern: "INSTÄLLT"
   ```

   Usage: `/query?Installt=true`

### Creating Your Own Custom Filters

Let's say you want to filter a corporate calendar by project codes (PROJ-001, PROJ-002, etc.) and priority levels.

**1. Define filters in config.yaml:**

```yaml
filters:
  # Project code filter
  project:
    field: "SUMMARY"
    pattern_template: "PROJ-%s"

  # Priority filter
  priority:
    field: "SUMMARY"
    patterns:
      "high":
        template: "\\[HIGH\\]"
      "medium":
        template: "\\[MEDIUM\\]"
      "low":
        template: "\\[LOW\\]"
      default:
        template: "\\[%s\\]"

  # Department filter
  dept:
    field: "LOCATION"
    pattern_template: "%s Department"
```

**2. Use in URLs:**

```
# Filter out PROJ-001 and PROJ-002
/query?project=001,002

# Keep only high priority
/query?priority=high

# Filter out Engineering department
/query?dept=Engineering

# Combine filters
/query?project=001&priority=high&dept=Engineering
```

### Filter Configuration Reference

#### Simple Pattern Template

```yaml
filter_name:
  field: "SUMMARY"              # iCal field to search
  pattern_template: "text %s"   # Pattern with %s placeholder
```

- `%s` is replaced with the value from the URL parameter
- Example: `?filter_name=foo` → matches "text foo"

#### Pattern Map (Multiple Values)

```yaml
filter_name:
  field: "SUMMARY"
  patterns:
    "value1":
      template: "pattern1"
    "value2":
      template: "pattern2"
    default:
      template: "default pattern %s"
```

- Maps specific values to custom patterns
- Falls back to `default` pattern if value not in map
- Example: `?filter_name=value1` → matches "pattern1"

#### Regex Escaping

Remember to escape regex special characters in patterns:

```yaml
# Literal brackets
pattern: "\\[Important\\]"

# Literal parentheses
pattern: "Meeting \\(Remote\\)"

# Literal dot
pattern: "v1\\.2\\.3"
```

## Real-World Examples

### 1. Corporate Calendar

Filter engineering meetings and high-priority items:

**config.yaml:**
```yaml
filters:
  team:
    field: "SUMMARY"
    pattern_template: "\\[%s\\]"

  priority:
    field: "SUMMARY"
    patterns:
      "p0":
        template: "P0"
      "p1":
        template: "P1"
      default:
        template: "P[0-9]"
```

**URLs:**
```
/query?team=Engineering,Sales
/query?priority=p0,p1
```

### 2. School Calendar

Filter by grade level and event type:

**config.yaml:**
```yaml
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade %s"

  event_type:
    field: "CATEGORIES"
    patterns:
      "sports":
        template: "Athletics|Sports|Game"
      "academic":
        template: "Test|Exam|Assignment"
      default:
        template: "%s"
```

**URLs:**
```
/query?grade=9,10,11,12
/query?event_type=sports
```

### 3. Multi-Location Business

Filter by office location:

**config.yaml:**
```yaml
filters:
  office:
    field: "LOCATION"
    patterns:
      "sf":
        template: "San Francisco|SF Office"
      "nyc":
        template: "New York|NYC Office"
      "ldn":
        template: "London|LDN Office"
      default:
        template: "%s Office"
```

**URLs:**
```
/query?office=sf,nyc
```

## Testing Your Custom Filters

Use debug mode to see what's being filtered:

```
/query?your_filter=value&debug=true
```

This shows:
- Which filters are active
- Which events matched
- What patterns were used

## Configuration Files

- **config.yaml.example** - Generic starting point
- **config-parbricole.yaml.example** - Par Bricole specific example
- **config.yaml** - Your actual config (not in git, contains secrets)

## Filter Behavior

**Important**: Normal filters **remove** matching events.

Exception: Some filters can be **inverted** to keep only matching events. This requires code changes (see `internal/filter/filter.go`).

## Next Steps

1. Copy `config.yaml.example` to `config.yaml`
2. Add your upstream iCal URL
3. Define custom filters (optional)
4. Test with debug mode: `/query?pattern=test&debug=true`
5. Deploy!

For deployment instructions, see [DEPLOY.md](DEPLOY.md).
