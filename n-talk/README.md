# n-talk

The purpose of this experiment is to develop a system that facilitates the talking between two autonomous AI systems. `n` represents an `n` number of participants in a conversation. Ideally, the goal should be greater-than-or-equal-to three participants.

## Implementation

The LLM services are service-agnostic. As long as adapters for a service are implemented, it can be used in the agent. LLM adapters are found in the `llms` package.

### Facilitation Methods

There are two implementable methods of facilitation. The first is **turn-taking**. The second is **message queue**.

#### Turn-taking

In a real-life conversation between humans, interlocutors take turns, essentially creating a conversational puzzle that interlocutors manifest together, both a puzzle and the solution. Turns can be analyzed with game theory.

Turn taking can be implemented with a simple sender-receiver mechanism in which an agent, agent _A_, initiates the topic and context of the conversation. A central server then relays this message to another agent, agent _B_, which interprets the message and crafts a response back. The central server then relays the response back to A. This dance continues until a definite end. The definite end can be instantiated by "feel", where the agents, amongst themselves, "feel" out the end of the conversation; or by "function", the central server sends out a pre-written message telling the agents that it is time to wrap up.

_Picture of state machine_

#### Message Queue

Message queue is useful when there are more than two agents. In real life, a conversation among three interlocutors is mediated by power dynamic, status, and most importantly, relationship. The relationship between interlocutors is implicitly understood, agreed upon, and validated. Of course, the relationships can change over the course of the conversation.

The relationships between interlocutors determines the number of addressees, the number of addressers, and the concurrent establishment of truths related to the conversational puzzle. A single interlocutor may first listen to the conversation of two other speakers and wait to chime in. Their interjection may address zero to `n - 1` speakers in the conversation (`n - 1` excludes themselves). It is individual choice, up to the discretion of the listener.

A message queue like Kafka or a subscription service like Redis can be implemented and used by a central server. Agents consume messages from the message queue, which is essentially just a queue, with elements duplicated, the duplicated messages total `n - 1`.

_Picture of state machine_

## Useful Links

- <https://ai.google.dev/gemini-api/docs/text-generation?lang=go>
- <https://aistudio.google.com/plan_information>

### Databases

- <https://withcodeexample.com/golang-postgres/>
- <https://donchev.is/post/working-with-postgresql-in-go-using-pgx/#inserting-data>
