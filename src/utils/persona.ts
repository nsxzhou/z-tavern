import type { Persona } from '../api'

export const getPersonaInitials = (persona: Persona) => {
  if (persona.initials) return persona.initials
  if (persona.name) return persona.name.slice(0, 2).toUpperCase()
  return 'AI'
}

export const getPersonaMood = (persona: Persona) => {
  const level = typeof persona.mood === 'number' ? persona.mood : 0.6
  const text = persona.moodText ?? '已就绪，随时展开新对话'
  return { level, text }
}

export const getPersonaPrompts = (persona: Persona): string[] => {
  if (Array.isArray(persona.prompts) && persona.prompts.length) return persona.prompts
  if (Array.isArray(persona.samplePrompts) && persona.samplePrompts.length) return persona.samplePrompts
  if (persona.promptHint) {
    return persona.promptHint
      .split(/[。！？!?]/)
      .map((segment) => segment.trim())
      .filter(Boolean)
      .slice(0, 4)
  }
  return []
}

export const createPersonaFilterTags = (personas: Persona[]) => {
  const tagSet = new Set<string>()
  personas.forEach((persona) => {
    ;(persona.expertise ?? persona.style ?? persona.traits ?? []).forEach((tag) => {
      if (tag) tagSet.add(tag)
    })
  })
  return Array.from(tagSet).slice(0, 12)
}
