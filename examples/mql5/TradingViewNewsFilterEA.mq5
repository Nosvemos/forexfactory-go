//+------------------------------------------------------------------+
//|                                  TradingViewNewsFilterEA.mq5     |
//|                    Copyright 2026, tradingview-calendar-go       |
//|                https://github.com/Nosvemos/tradingview-calendar-go|
//+------------------------------------------------------------------+
#property copyright "TradingView-Calendar-Go"
#property link      "https://github.com/Nosvemos/tradingview-calendar-go"
#property version   "1.00"

#include <TradingViewNewsFilter.mqh>

//--- Input Parameters
input int    InpMinutesBefore = 30;       // Stop trading X minutes BEFORE news
input int    InpMinutesAfter  = 15;       // Resume trading X minutes AFTER news
input string InpMinImpact     = "High";   // Minimum News Impact (High / Medium / Low)
input string InpFileName      = "ff_news_filter.csv"; // Published news filter filename

//--- Global Instance
CTradingViewNewsFilter NewsFilter(InpFileName);

//+------------------------------------------------------------------+
//| Expert initialization function                                   |
//+------------------------------------------------------------------+
int OnInit()
{
   Print("[MQL5 EA] TradingView News Filter EA initialized on symbol: ", _Symbol);
   Print("[MQL5 EA] Protection Window: -", InpMinutesBefore, " min to +", InpMinutesAfter, " min (Threshold: ", InpMinImpact, ")");
   return(INIT_SUCCEEDED);
}

//+------------------------------------------------------------------+
//| Expert deinitialization function                                 |
//+------------------------------------------------------------------+
void OnDeinit(const int reason)
{
   Print("[MQL5 EA] News Filter EA terminated.");
}

//+------------------------------------------------------------------+
//| Expert tick function                                             |
//+------------------------------------------------------------------+
void OnTick()
{
   // Check if trading is restricted due to upcoming or recently released high impact news
   if(NewsFilter.IsNewsRestricted(_Symbol, InpMinutesBefore, InpMinutesAfter, InpMinImpact))
   {
      Comment("🔴 TRADING RESTRICTED: Active news risk on ", _Symbol);
      return;
   }

   Comment("🟢 TRADING ALLOWED: No high-impact news within safety window.");

   // --- Place Expert Advisor order management logic here ---
}
