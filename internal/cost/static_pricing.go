package cost

func Price(model string, promptTokens int, completionTokens int) float64 {
	return StaticPricer{}.Price(model, promptTokens, completionTokens)
}

type StaticPricer struct{}

func (StaticPricer) Price(model string, promptTokens int, completionTokens int) float64 {
	price := priceForModel(model)
	return (float64(promptTokens)*price.inputPerMTok + float64(completionTokens)*price.outputPerMTok) / 1_000_000
}

type modelPrice struct {
	inputPerMTok  float64
	outputPerMTok float64
}

func priceForModel(model string) modelPrice {
	if price, ok := defaultPrices()[model]; ok {
		return price
	}
	return modelPrice{inputPerMTok: 1.00, outputPerMTok: 3.00}
}
