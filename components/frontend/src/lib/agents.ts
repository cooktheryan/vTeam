import { AgentPersona } from "@/types/agentic-session";

// Agent persona definitions based on the YAML files in claude-runner
export const AVAILABLE_AGENTS: AgentPersona[] = [
  {
    persona: "ENGINEERING_MANAGER",
    name: "Emma (Engineering Manager)",
    role: "Engineering Management",
    expertise: ["team-leadership", "capacity-planning", "delivery-coordination", "technical-strategy"],
    description: "Focuses on team wellbeing, sustainable delivery practices, and balancing technical excellence with business needs."
  },
  {
    persona: "STAFF_ENGINEER",
    name: "Stella (Staff Engineer)",
    role: "Technical Leadership",
    expertise: ["technical-leadership", "implementation-excellence", "code-quality", "performance-optimization"],
    description: "Bridges architectural vision to practical implementation, champions code quality, and mentors teams through complex technical challenges."
  },
  {
    persona: "PRODUCT_MANAGER",
    name: "Parker (Product Manager)",
    role: "Product Strategy",
    expertise: ["product-strategy", "market-analysis", "stakeholder-management", "feature-prioritization"],
    description: "Drives product vision, manages stakeholder relationships, and ensures features align with business objectives."
  },
  {
    persona: "TEAM_LEAD",
    name: "Lee (Team Lead)",
    role: "Team Coordination",
    expertise: ["team-coordination", "sprint-planning", "technical-mentoring", "cross-functional-collaboration"],
    description: "Coordinates team execution, facilitates technical discussions, and ensures smooth delivery of features."
  },
  {
    persona: "SCRUM_MASTER",
    name: "Sam (Scrum Master)",
    role: "Process Facilitation",
    expertise: ["agile-methodologies", "process-improvement", "team-facilitation", "impediment-removal"],
    description: "Facilitates agile processes, removes blockers, and helps teams continuously improve their delivery practices."
  },
  {
    persona: "UX_ARCHITECT",
    name: "Aria (UX Architect)",
    role: "User Experience Design",
    expertise: ["ux-architecture", "design-systems", "user-research", "accessibility"],
    description: "Designs comprehensive user experiences, establishes design systems, and ensures accessibility standards."
  },
  {
    persona: "UX_TEAM_LEAD",
    name: "Uma (UX Team Lead)",
    role: "UX Team Leadership",
    expertise: ["ux-leadership", "design-strategy", "team-mentoring", "stakeholder-communication"],
    description: "Leads UX teams, coordinates design strategy, and ensures design quality across products."
  },
  {
    persona: "UX_FEATURE_LEAD",
    name: "Felix (UX Feature Lead)",
    role: "Feature UX Leadership",
    expertise: ["feature-design", "user-journey-mapping", "interaction-design", "usability-testing"],
    description: "Leads UX design for specific features, maps user journeys, and validates design decisions through testing."
  },
  {
    persona: "UX_RESEARCHER",
    name: "Ryan (UX Researcher)",
    role: "User Research",
    expertise: ["user-research", "data-analysis", "usability-testing", "behavioral-insights"],
    description: "Conducts user research, analyzes behavioral data, and provides insights to inform design decisions."
  },
  {
    persona: "CONTENT_STRATEGIST",
    name: "Casey (Content Strategist)",
    role: "Content Strategy",
    expertise: ["content-strategy", "information-architecture", "content-planning", "editorial-guidelines"],
    description: "Develops content strategies, organizes information architecture, and ensures consistent messaging."
  },
  {
    persona: "TECHNICAL_WRITER",
    name: "Terry (Technical Writer)",
    role: "Technical Documentation",
    expertise: ["technical-writing", "documentation-strategy", "api-documentation", "user-guides"],
    description: "Creates comprehensive technical documentation, API guides, and user-facing documentation."
  },
  {
    persona: "TECHNICAL_WRITING_MANAGER",
    name: "Tessa (Technical Writing Manager)",
    role: "Documentation Management",
    expertise: ["documentation-management", "technical-communication", "content-governance", "team-leadership"],
    description: "Manages technical writing teams, establishes documentation standards, and oversees content quality."
  },
  {
    persona: "DELIVERY_OWNER",
    name: "Derek (Delivery Owner)",
    role: "Release Coordination",
    expertise: ["release-management", "delivery-coordination", "stakeholder-communication", "risk-management"],
    description: "Coordinates feature delivery, manages release schedules, and ensures stakeholder alignment."
  },
  {
    persona: "PXE",
    name: "Phoenix (PXE Specialist)",
    role: "Platform Experience",
    expertise: ["platform-experience", "developer-experience", "tooling", "infrastructure"],
    description: "Focuses on platform and developer experience, tooling improvements, and infrastructure usability."
  }
];

// Default agent selections for different workflow types
export const DEFAULT_AGENT_SELECTIONS = {
  BALANCED: ["ENGINEERING_MANAGER", "STAFF_ENGINEER", "PRODUCT_MANAGER", "TEAM_LEAD"],
  TECHNICAL: ["STAFF_ENGINEER", "ENGINEERING_MANAGER", "UX_ARCHITECT", "TECHNICAL_WRITER"],
  PRODUCT: ["PRODUCT_MANAGER", "UX_RESEARCHER", "CONTENT_STRATEGIST", "DELIVERY_OWNER"],
  DESIGN: ["UX_ARCHITECT", "UX_FEATURE_LEAD", "UX_RESEARCHER", "CONTENT_STRATEGIST"],
  PROCESS: ["SCRUM_MASTER", "ENGINEERING_MANAGER", "DELIVERY_OWNER", "TEAM_LEAD"]
};

// Utility functions
export function getAgentByPersona(persona: string): AgentPersona | undefined {
  return AVAILABLE_AGENTS.find(agent => agent.persona === persona);
}

export function getDefaultAgents(type: keyof typeof DEFAULT_AGENT_SELECTIONS = "BALANCED"): AgentPersona[] {
  return DEFAULT_AGENT_SELECTIONS[type].map(persona => getAgentByPersona(persona)!).filter(Boolean);
}

export function getAgentsByExpertise(expertise: string): AgentPersona[] {
  return AVAILABLE_AGENTS.filter(agent =>
    agent.expertise.some(exp => exp.includes(expertise) || expertise.includes(exp))
  );
}

export function groupAgentsByRole(): { [role: string]: AgentPersona[] } {
  const groups: { [role: string]: AgentPersona[] } = {};

  AVAILABLE_AGENTS.forEach(agent => {
    const category = getCategoryForRole(agent.role);
    if (!groups[category]) {
      groups[category] = [];
    }
    groups[category].push(agent);
  });

  return groups;
}

function getCategoryForRole(role: string): string {
  if (role.includes("Engineering") || role.includes("Technical")) return "Engineering";
  if (role.includes("UX") || role.includes("Design")) return "Design";
  if (role.includes("Product") || role.includes("Strategy")) return "Product";
  if (role.includes("Content") || role.includes("Documentation")) return "Content";
  return "Process & Leadership";
}

export const WORKFLOW_PHASE_LABELS = {
  pre: "⏳ Pre",
  ideate: "💡 Ideate",
  specify: "📝 Specify",
  plan: "🗂️ Plan",
  tasks: "✅ Tasks",
  review: "👁️ Review",
  completed: "🎉 Completed"
};

export const WORKFLOW_PHASE_DESCRIPTIONS = {
  ideate: "Collaboratively ideate and define the high-level RFE in rfe.md",
  specify: "Create comprehensive specifications from different perspectives",
  plan: "Generate detailed implementation plans with technical approach",
  tasks: "Break down features into actionable development tasks",
  review: "Review and finalize all artifacts before implementation",
  completed: "All phases complete, artifacts pushed to repository"
};