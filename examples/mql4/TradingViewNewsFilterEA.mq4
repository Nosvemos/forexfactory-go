//+------------------------------------------------------------------+
//|                                  TradingViewNewsFilterEA.mq4     |
//|                    Copyright 2026, tradingview-calendar-go       |
//|                https://github.com/Nosvemos/tradingview-calendar-go|
//+------------------------------------------------------------------+
#property copyright "TradingView-Calendar-Go"
#property link      "https://github.com/Nosvemos/tradingview-calendar-go"
#property version   "1.00"
#property strict

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
   Print("[EA Init] TradingView News Filter EA initialized on symbol: ", _Symbol);
   Print("[EA Init] News restrictions: ", InpMinutesBefore, " min before, ", InpMinutesAfter, " min after (Min Impact: ", InpMinImpact, ")");
   return(INIT_SUCCEEDED);
}

//+------------------------------------------------------------------+
//| Expert deinitialization function                                 |
//+------------------------------------------------------------------+
void OnDeinit(const int reason)
{
   Print("[EA Deinit] News Filter EA stopped.");
}

//+------------------------------------------------------------------+
//| Expert tick function                                             |
//+------------------------------------------------------------------+
void OnTick()
{
   // Check if trading is restricted due to upcoming or recently released high impact news
   if(NewsFilter.IsNewsRestricted(_Symbol, InpMinutesBefore, InpMinutesAfter, InpMinImpact))
   {
      Comment("Trading PAUSED: High impact news event detected on ", _Symbol);
      return;
   }

   Comment("Trading ACTIVE: No restricting news events.");

   // --- Normal EA Trading Logic here ---
}
