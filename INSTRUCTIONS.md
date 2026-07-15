# Technical Challenge

:wave: Welcome to the Ctrl Hub technical challenge! This repo contains a technical challenge to complete as part of our hiring process.

## Brief

The goal of the challenge is to create a runnable HTTP API in Golang which is specified in [`spec.yaml`](./spec.yaml).

The API is a simple domain. It allows clients to submit exposure to equipment that can cause Hand and Arm Vibrating Syndrome (HAVS) on behalf of a user. The API allows clients to submit exposures and retrieve a user's total exposure within a timeframe (usually 24 hour windows).

If you are unfamiliar with HAVS, there is some background information from the [HSE](https://www.hse.gov.uk/vibration/hav/index.htm) you can read.

The general business premise is that we have users who use equipment items on a daily basis. Different equipment has different  vibration magnitudes that, depending on the duration of use, contribute to the HAVS score of a user. There are two numbers that we care about when summarising a user's exposure - the "Partial Exposure Points" and the "Partial Exposure A(8)".

So that you do not need to reverse engineer the calculations the HSE look for, here is the calculation expressed as a function which you may choose to use as is, or move to a more appropriate location in your codebase (for example, as a method on your `Exposure` entity). Both of these functions take two parameters - the vibration magnitude of the equipment item being used (measured in ms/2), and the duration of use (measured in minutes):

```golang
func partialExposureA8(vibrationMagnitude float64, triggerTime int) float64 {
	return vibrationMagnitude * math.Sqrt((triggerTime / 60) / 8)
}

func partialExposurePoints(vibrationMagnitude float64, triggerTime int) float64 {
	points := math.Pow((vibrationMagnitude / 2.5), 2) * (((triggerTime / 60) / 8) * 100)
	return math.Round(points)
}
```

And here are two pieces of equipment that have a vibration magnitude:

- An "AirCat Drill (model 4337)" with a vibration magnitude of 2.1 ms/2
- A "JCB Hydraulic Breaker (model CEJCBHM25)" with a vibration magnitude of 4.0 ms/2

## Entities / Models

The entities are contained in the spec file, within the schema/components section. There are entities for a `User`, `EquipmentItem`, `Exposure` and `ExposureSummary`.

## What we're looking for

A solution which satisfies the spec and demonstrates your understanding of the problem and your ability to write clean, maintainable code.

Responses can either be a public repository or a zip file containing the solution. Ideally, the solution should be runnable with a single command (e.g. docker compose)

Please include a README with instructions on how to run the solution.

You should consider as part of your solution:

 - Domain Driven Design
 - Clean architectural patterns
 - Test coverage (don't aim for 100% coverage, but do try and show us how you test your code)

We don't want to burden you with a time consuming task, so we don't expect you to spend more than 4 hours on this challenge. If you don't finish in time, don't worry! Just submit what you have and we can discuss it in the followup.

If compromises need to be made due to time constraints, please document them in your response README. They are perfectly fine to make, but where you can signal what you would have done given more time (ie a real world scenario), you should try and do so.

## Persistence

The persistence layer is entirely for you to choose. You can choose to persist in memory for the purposes of the test, or you may choose to use a database. If you use a database, please ensure that the solution is runnable with a single command. The choice of database is entirely up to you - we primarily use Mongo, Postgres and MySQL but feel free to choose a technology you are comfortable with.

## Events

Whilst not specified, please give some consideration to how the solution would fit into an Event Drive Architecture, what kinds of data you may be interested in publishing and any you may be interested in consuming. If you can summarise these in your README, we will discuss them in a followup.

## Frameworks vs stdlib

Whilst we always prefer to be as close to stdlib as possible, for the purposes of this test, feel free to include frameworks and libraries that can help. The main constraint of this challenge is time, and we acknowledge that adopting third party libraries may have an impact on performance. We are more interested in exploring your approach to solving the domain issues than to the "pureness" of the solution.

## Final thoughts

We appreciate your time taken to complete this challenge. We understand that it is a significant ask, and we are grateful for your time. If you have any questions, please don't hesitate to ask.

We have tried to make the test as close to a real world example of the type of daily challenges we face at Ctrl Hub. Whilst the spec is overly simplfied compared to what we would have in production, we hope you enjoy the challenge.

Good luck! :rocket:
