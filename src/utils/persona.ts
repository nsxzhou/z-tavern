import type { Persona } from '../api'

export const getPersonaInitials = (persona: Persona) => {
  if (persona.name) return persona.name.slice(0, 2).toUpperCase()
  return 'AI'
}


export const createPersonaFilterTags = (personas: Persona[]) => {
  const tagSet = new Set<string>()
  personas.forEach((persona) => {
    ;(persona.traits ?? []).forEach((tag) => {
      if (tag) tagSet.add(tag)
    })
    ;(persona.expertise ?? []).forEach((tag) => {
      if (tag) tagSet.add(tag)
    })
  })
  return Array.from(tagSet).slice(0, 12)
}
